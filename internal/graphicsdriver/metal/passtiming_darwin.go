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

package metal

import (
	"time"

	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver/metal/mtl"
)

var _ graphicsdriver.PassTimer = (*Graphics)(nil)

// passTimes holds the render passes of the frame that the render-pass log times. Only the render
// thread uses it.
type passTimes struct {
	// on reports whether the current frame is the timed frame: each new render pass starts a new
	// command buffer.
	on bool

	// pending reports whether the GPU times of frame are not read yet.
	pending bool

	frame   int64
	records []passRecord
}

type passRecord struct {
	cb   mtl.CommandBuffer
	time graphicsdriver.PassTime
}

// TimePasses implements graphicsdriver.PassTimer.
func (g *Graphics) TimePasses() bool {
	g.passTimes = passTimes{on: true, pending: true, frame: g.frame, records: g.passTimes.records[:0]}
	return true
}

// ReadPassTimes implements graphicsdriver.PassTimer.
func (g *Graphics) ReadPassTimes(dst []graphicsdriver.PassTime) ([]graphicsdriver.PassTime, bool) {
	if g.passTimes.on || g.passTimes.pending {
		return dst, false
	}
	for _, r := range g.passTimes.records {
		dst = append(dst, r.time)
	}
	return dst, true
}

// waitForCommandBuffers waits until the GPU completes every command buffer that is not released
// yet, including the command buffers of earlier frames. In the timed frame, the driver calls it
// after each commit, so that the GPU runs each timed render pass alone: command buffers that the
// GPU runs at the same time give each other's work to their GPU times. Every command buffer
// except g.cb is committed, and the caller must commit g.cb first.
func (g *Graphics) waitForCommandBuffers() {
	for _, cbs := range g.frameToCB {
		for _, cb := range cbs {
			cb.WaitUntilCompleted()
		}
	}
}

// add records a render pass to dst in the current command buffer.
func (p *passTimes) add(cb mtl.CommandBuffer, dst *Image) {
	p.records = append(p.records, passRecord{
		cb: cb,
		time: graphicsdriver.PassTime{
			Dst:    dst.id,
			Width:  dst.width,
			Height: dst.height,
		},
	})
}

// end stops the timing at the end of frame.
func (p *passTimes) end(frame int64, hasCommandBuffers bool) {
	if !p.on || p.frame != frame {
		return
	}
	p.on = false
	if !hasCommandBuffers {
		p.pending = false
	}
}

// complete reads the GPU times of the passes of frame, whose command buffers are all completed and
// not released yet.
func (p *passTimes) complete(frame int64) {
	if !p.pending || p.on || p.frame != frame {
		return
	}
	for i := range p.records {
		r := &p.records[i]
		if start, end := r.cb.GPUStartTime(), r.cb.GPUEndTime(); end > start {
			r.time.GPU = time.Duration((end - start) * float64(time.Second))
		}
		r.cb = mtl.CommandBuffer{}
	}
	p.pending = false
}
