// Copyright 2026 The Ebitengine Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build !playstation5

package opengl

import (
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver/opengl/gl"
)

// A timer query measures the GPU time of each flush, from Begin to End, and the GPU time of a frame
// is the sum of its flushes. The driver reads the results some frames later, when the GPU has them,
// and never waits for them. In the frame that the render-pass log captures, a timer query measures
// each render pass in place of the flushes, as only one timer query can run at a time.

var (
	_ graphicsdriver.FrameTimer = (*Graphics)(nil)
	_ graphicsdriver.PassTimer  = (*Graphics)(nil)
)

// frameQueryCount is the count of the timer queries of the flushes. A frame usually has one or two
// flushes, and the GPU is a few frames behind.
const frameQueryCount = 128

type frameQuery struct {
	id    uint32
	frame int64

	// last reports whether the query is the last one of its frame.
	last bool
}

// gpuTimer times the frames and the render passes. Only the render thread uses it, except
// supported and timings.
type gpuTimer struct {
	// supported reports whether the context has timer queries.
	supported atomic.Bool

	timings graphicsdriver.FrameTimings

	// queries is a ring of the timer queries of the flushes. The pending queries start at head.
	queries [frameQueryCount]frameQuery
	created bool
	head    int
	pending int

	// active reports whether the query of the current flush runs.
	active bool

	// lastDst is the destination of the current render pass, or nil.
	lastDst *Image

	passes passTimes
}

// passTimes holds the render passes of the frame that the render-pass log captures.
type passTimes struct {
	// on reports whether the current frame is the timed frame.
	on bool

	// pending reports whether the GPU times of the timed frame are not read yet.
	pending bool

	// open reports whether the query of the current render pass runs.
	open bool

	frame   int64
	queries []uint32
	records []graphicsdriver.PassTime
}

// ReadFrameTimings implements graphicsdriver.FrameTimer. Without timer queries, it returns 0 and -1.
func (g *Graphics) ReadFrameTimings(dst []graphicsdriver.FrameTiming) (int, int64) {
	if !g.timer.supported.Load() {
		return 0, -1
	}
	return g.timer.timings.Read(dst)
}

// TimePasses implements graphicsdriver.PassTimer.
func (g *Graphics) TimePasses() bool {
	if !g.timer.supported.Load() {
		return false
	}
	p := &g.timer.passes
	p.on, p.pending, p.open = true, true, false
	p.frame = g.frame
	p.records = p.records[:0]
	return true
}

// ReadPassTimes implements graphicsdriver.PassTimer.
func (g *Graphics) ReadPassTimes(dst []graphicsdriver.PassTime) ([]graphicsdriver.PassTime, bool) {
	p := &g.timer.passes
	if p.on {
		return dst, false
	}
	if p.pending {
		ctx := g.context.ctx
		if n := len(p.records); n > 0 {
			// The results of the earlier queries are available when the result of the last one is.
			if ctx.GetQueryObjectui(p.queries[n-1], gl.QUERY_RESULT_AVAILABLE) == 0 {
				return dst, false
			}
			disjoint := g.disjoint()
			for i := range p.records {
				ns := ctx.GetQueryObjectui(p.queries[i], gl.QUERY_RESULT)
				if !disjoint {
					p.records[i].GPU = time.Duration(ns)
				}
			}
		}
		p.pending = false
	}
	return append(dst, p.records...), true
}

// disjoint reports whether an event on OpenGL ES, such as a change of the GPU clock, makes the
// results of the timer queries invalid. The check clears the flag.
func (g *Graphics) disjoint() bool {
	return g.context.ctx.IsES() && g.context.ctx.GetInteger(gl.GPU_DISJOINT_EXT) != 0
}

// beginTiming reads the finished timer queries and starts the query of a flush. Call it at Begin.
func (g *Graphics) beginTiming() {
	t := &g.timer
	if !t.timings.On() {
		return
	}
	t.timings.Begin(g.frame, 0)
	g.readFrameQueries()
	if t.passes.on || t.pending == frameQueryCount {
		return
	}
	ctx := g.context.ctx
	if !t.created {
		for i := range t.queries {
			t.queries[i].id = ctx.CreateQuery()
		}
		t.created = true
	}
	q := &t.queries[(t.head+t.pending)%frameQueryCount]
	q.frame, q.last = g.frame, false
	ctx.BeginQuery(gl.TIME_ELAPSED, q.id)
	t.pending++
	t.active = true
}

// readFrameQueries adds the results of the finished queries of the flushes to their frames, in
// order, and stops at the first query that the GPU has not finished.
func (g *Graphics) readFrameQueries() {
	t := &g.timer
	ctx := g.context.ctx
	var checked, disjoint bool
	for t.pending > 0 {
		q := &t.queries[t.head]
		if ctx.GetQueryObjectui(q.id, gl.QUERY_RESULT_AVAILABLE) == 0 {
			return
		}
		if !checked {
			disjoint = g.disjoint()
			checked = true
		}
		ns := ctx.GetQueryObjectui(q.id, gl.QUERY_RESULT)
		if !disjoint {
			t.timings.AddGPUTime(q.frame, time.Duration(ns))
		}
		if q.last {
			t.timings.Finish(q.frame)
		}
		t.head = (t.head + 1) % frameQueryCount
		t.pending--
	}
}

// endTiming ends the query of a flush and the current render pass. At the end of a frame, it marks
// the last query of the frame, or finishes a frame without a query. Call it at End before the
// frame number changes.
func (g *Graphics) endTiming(mode graphicsdriver.FlushMode) {
	g.endPass()
	t := &g.timer
	if t.active {
		g.context.ctx.EndQuery(gl.TIME_ELAPSED)
		t.active = false
	}
	if mode == graphicsdriver.FlushModeIntermediate {
		return
	}
	if t.passes.on && t.passes.frame == g.frame {
		t.passes.on = false
	}
	if !t.timings.On() {
		return
	}
	if q := &t.queries[(t.head+t.pending+frameQueryCount-1)%frameQueryCount]; t.pending > 0 && q.frame == g.frame {
		q.last = true
	} else {
		t.timings.Finish(g.frame)
	}
	t.timings.End(g.frame)
}

// beginPass counts a new render pass for a draw to dst, and starts its query in the timed frame.
// The passes are those of the render-pass log: a new pass when the destination changes, as the
// Metal driver starts a new render command encoder then.
func (g *Graphics) beginPass(dst *Image) {
	t := &g.timer
	if t.lastDst == dst {
		return
	}
	g.endPass()
	t.lastDst = dst
	if t.timings.On() {
		t.timings.AddPass(g.frame)
	}
	p := &t.passes
	if !p.on {
		return
	}
	i := len(p.records)
	if i == len(p.queries) {
		p.queries = append(p.queries, g.context.ctx.CreateQuery())
	}
	g.context.ctx.BeginQuery(gl.TIME_ELAPSED, p.queries[i])
	p.records = append(p.records, graphicsdriver.PassTime{Dst: dst.id, Width: dst.width, Height: dst.height})
	p.open = true
}

// endPass ends the current render pass: at the end of a flush, and before a pixel read or write.
func (g *Graphics) endPass() {
	t := &g.timer
	t.lastDst = nil
	if t.passes.open {
		g.context.ctx.EndQuery(gl.TIME_ELAPSED)
		t.passes.open = false
	}
}

// resetTiming forgets the queries of a lost context. The frames that wait for them go out without a
// GPU time.
func (g *Graphics) resetTiming() {
	t := &g.timer
	t.created, t.head, t.pending, t.active, t.lastDst = false, 0, 0, false, nil
	t.passes = passTimes{}
}
