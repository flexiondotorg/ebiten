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
	"testing"
	"time"

	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver/opengl/gl"
)

// queryContext is a context with timer queries. Each query measures 1 ms more than the one before,
// and its result is available once the GPU reaches it.
type queryContext struct {
	gl.Context

	next   uint32
	active uint32
	ns     map[uint32]uint32

	// reached is the count of the queries that the GPU finished, in the order of BeginQuery.
	reached int
	order   []uint32
}

func newQueryContext() *queryContext {
	return &queryContext{ns: make(map[uint32]uint32, 1024), order: make([]uint32, 0, 1024)}
}

func (c *queryContext) IsES() bool { return false }

func (c *queryContext) HasTimerQuery() bool { return true }

func (c *queryContext) CreateQuery() uint32 {
	c.next++
	return c.next
}

func (c *queryContext) BeginQuery(target uint32, query uint32) {
	if c.active != 0 {
		panic("a timer query runs already")
	}
	c.active = query
	c.order = append(c.order, query)
	c.ns[query] = uint32(len(c.order)) * uint32(time.Millisecond)
}

func (c *queryContext) EndQuery(target uint32) {
	if c.active == 0 {
		panic("no timer query runs")
	}
	c.active = 0
}

func (c *queryContext) GetQueryObjectui(query uint32, pname uint32) uint32 {
	switch pname {
	case gl.QUERY_RESULT_AVAILABLE:
		for _, q := range c.order[:c.reached] {
			if q == query {
				return 1
			}
		}
		return 0
	case gl.QUERY_RESULT:
		return c.ns[query]
	}
	return 0
}

// finishGPU lets the GPU reach every query that started.
func (c *queryContext) finishGPU() {
	c.reached = len(c.order)
}

func newTimedGraphics() (*Graphics, *queryContext) {
	c := newQueryContext()
	g := &Graphics{}
	g.context.ctx = c
	g.timer.supported.Store(true)
	return g, c
}

// runTimedFrame runs a frame with two flushes, and in them three render passes, one with depth.
func runTimedFrame(g *Graphics, a, b *Image) {
	g.beginTiming()
	g.beginPass(a, false)
	g.beginPass(a, false)
	g.beginPass(b, true)
	g.endTiming(graphicsdriver.FlushModeIntermediate)
	g.beginTiming()
	g.beginPass(b, false)
	g.endTiming(graphicsdriver.FlushModeEndFrame)
	g.frame++
}

func TestFrameTimings(t *testing.T) {
	g, c := newTimedGraphics()
	a := &Image{id: 1, width: 64, height: 32}
	b := &Image{id: 2, width: 128, height: 64}
	var dst [8]graphicsdriver.FrameTiming
	g.ReadFrameTimings(dst[:])

	// Frame 0 runs while the timing turns on, so it is not read.
	runTimedFrame(g, a, b)
	runTimedFrame(g, a, b)
	if n, frame := g.ReadFrameTimings(dst[:]); n != 0 || frame != 1 {
		t.Fatalf("before the GPU finishes: got %d records and frame %d, want 0 and 1", n, frame)
	}

	// The queries of the frames are the 1st to the 4th: frame 1 has the 3rd and the 4th.
	c.finishGPU()
	runTimedFrame(g, a, b)
	n, _ := g.ReadFrameTimings(dst[:])
	if n != 1 {
		t.Fatalf("got %d records, want 1", n)
	}
	want := graphicsdriver.FrameTiming{Frame: 1, GPU: 7 * time.Millisecond, Passes: 3, DepthPasses: 1}
	if dst[0] != want {
		t.Errorf("got %+v, want %+v", dst[0], want)
	}
}

func TestPassTimes(t *testing.T) {
	g, c := newTimedGraphics()
	a := &Image{id: 1, width: 64, height: 32}
	b := &Image{id: 2, width: 128, height: 64}
	var dst [8]graphicsdriver.FrameTiming
	g.ReadFrameTimings(dst[:])
	runTimedFrame(g, a, b)

	if !g.TimePasses() {
		t.Fatal("TimePasses reported false")
	}
	runTimedFrame(g, a, b)
	if _, ok := g.ReadPassTimes(nil); ok {
		t.Fatal("got the pass times before the GPU finished")
	}
	c.finishGPU()
	times, ok := g.ReadPassTimes(nil)
	if !ok {
		t.Fatal("got no pass times after the GPU finished")
	}
	// The frame queries of frame 0 are the 1st and the 2nd, and the passes of frame 1 the 3rd to
	// the 5th.
	want := []graphicsdriver.PassTime{
		{Dst: 1, Width: 64, Height: 32, GPU: 3 * time.Millisecond},
		{Dst: 2, Width: 128, Height: 64, Depth: true, GPU: 4 * time.Millisecond},
		{Dst: 2, Width: 128, Height: 64, GPU: 5 * time.Millisecond},
	}
	if len(times) != len(want) {
		t.Fatalf("got %+v, want %+v", times, want)
	}
	for i := range want {
		if times[i] != want[i] {
			t.Errorf("pass %d: got %+v, want %+v", i+1, times[i], want[i])
		}
	}

	// The timed frame has no frame query, so it goes out without a GPU time.
	runTimedFrame(g, a, b)
	c.finishGPU()
	runTimedFrame(g, a, b)
	n, _ := g.ReadFrameTimings(dst[:])
	if n < 2 || dst[0].Frame != 1 || dst[0].GPU != 0 || dst[0].Passes != 3 || dst[1].Frame != 2 || dst[1].GPU == 0 {
		t.Errorf("got %+v, want frame 1 without a GPU time and frame 2 with one", dst[:n])
	}
}

func TestFrameTimingsWithoutTimerQueries(t *testing.T) {
	g := &Graphics{}
	var dst [8]graphicsdriver.FrameTiming
	if n, frame := g.ReadFrameTimings(dst[:]); n != 0 || frame != -1 {
		t.Errorf("got %d records and frame %d, want 0 and -1", n, frame)
	}
	if g.TimePasses() {
		t.Error("TimePasses reported true")
	}
}

func TestFrameTimingsAllocations(t *testing.T) {
	g, c := newTimedGraphics()
	a := &Image{id: 1, width: 64, height: 32}
	b := &Image{id: 2, width: 128, height: 64}
	var dst [8]graphicsdriver.FrameTiming
	g.ReadFrameTimings(dst[:])
	runTimedFrame(g, a, b)
	allocs := testing.AllocsPerRun(100, func() {
		runTimedFrame(g, a, b)
		c.finishGPU()
		g.ReadFrameTimings(dst[:])
	})
	if allocs != 0 {
		t.Errorf("got %v allocations a frame, want 0", allocs)
	}
}
