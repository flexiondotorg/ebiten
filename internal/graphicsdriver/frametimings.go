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

package graphicsdriver

import (
	"sync"
	"sync/atomic"
	"time"
)

// frameTimingCount is the count of frame records that a driver keeps. It is the count of
// uncompleted command buffers that the default Metal command queue allows.
const frameTimingCount = 64

// FrameTimings is a fixed ring of the GPU timings of the latest frames, . A driver that implements FrameTimer keeps one. The render thread writes it,
// and the game goroutine reads it.
type FrameTimings struct {
	// on is set by the first read. Until then, the driver times nothing.
	on atomic.Bool

	// ended is the number of the latest ended frame plus one, or 0 before the first frame ends.
	ended atomic.Int64

	mu      sync.Mutex
	records [frameTimingCount]frameRecord

	// next is the number of the oldest frame that is not read yet.
	next int64

	// busyEnd is the latest GPU end host time of the command buffers that AddGPU added.
	busyEnd float64
}

type frameRecord struct {
	timing   FrameTiming
	finished bool
}

// record returns the record of frame, and resets it when it holds an older frame. Hold mu.
func (f *FrameTimings) record(frame int64) *frameRecord {
	r := &f.records[frame%frameTimingCount]
	if r.timing.Frame != frame {
		*r = frameRecord{timing: FrameTiming{Frame: frame}}
	}
	return r
}

// find returns the record of frame, or nil when the ring holds it no more.
func (f *FrameTimings) find(frame int64) *frameRecord {
	r := &f.records[frame%frameTimingCount]
	if r.timing.Frame != frame {
		return nil
	}
	return r
}

// On reports whether the timing is on: the first Read turns it on.
func (f *FrameTimings) On() bool {
	return f.on.Load()
}

// Begin stores the host time of the start of frame in seconds, once a frame.
func (f *FrameTimings) Begin(frame int64, start float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.record(frame); r.timing.Start == 0 {
		r.timing.Start = start
	}
}

// AddPass counts a render pass of frame, and its depth attachment.
func (f *FrameTimings) AddPass(frame int64, depth bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	r := f.record(frame)
	r.timing.Passes++
	if depth {
		r.timing.DepthPasses++
	}
}

// AddQueueWait adds a wait for a command buffer to frame.
func (f *FrameTimings) AddQueueWait(frame int64, d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.record(frame).timing.QueueWait += d
}

// AddGPU adds the GPU start and end host times of a completed command buffer of frame, in
// seconds. Call it in the order of the command buffers on the queue.
//
// The GPU runs the command buffers of a queue with overlaps, and the span of one command buffer
// covers the work of the others that run with it. So the GPU time counts only the part of the span
// after the end of the earlier command buffers, and the sum of the GPU times of the frames is the
// time that the GPU was busy.
func (f *FrameTimings) AddGPU(frame int64, start, end float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if end < start {
		return
	}
	from := max(start, f.busyEnd)
	f.busyEnd = max(f.busyEnd, end)
	r := f.find(frame)
	if r == nil {
		return
	}
	t := &r.timing
	if end > from {
		t.GPU += time.Duration((end - from) * float64(time.Second))
	}
	if t.GPUStart == 0 || start < t.GPUStart {
		t.GPUStart = start
	}
	t.GPUEnd = max(t.GPUEnd, end)
}

// AddGPUTime adds the GPU time of a part of frame that the GPU completed, for a driver that
// measures a time and no start and end. GPUStart and GPUEnd stay at zero.
func (f *FrameTimings) AddGPUTime(frame int64, d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.find(frame); r != nil {
		r.timing.GPU += d
	}
}

// Finish marks frame as finished: the GPU completed all its command buffers.
func (f *FrameTimings) Finish(frame int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if r := f.find(frame); r != nil {
		r.finished = true
	}
}

// End records that frame ended.
func (f *FrameTimings) End(frame int64) {
	f.ended.Store(frame + 1)
}

// Read implements FrameTimer.ReadFrameTimings. The first call turns the timing on
// and returns no records, and the frame that runs then is not read, as its timing is partial.
func (f *FrameTimings) Read(dst []FrameTiming) (int, int64) {
	ended := f.ended.Load() - 1
	f.mu.Lock()
	defer f.mu.Unlock()
	if !f.on.Load() {
		f.next = ended + 2
		f.on.Store(true)
		return 0, ended
	}
	var n int
	for ; n < len(dst) && f.next <= ended; n++ {
		r := f.find(f.next)
		switch {
		case r != nil && r.finished:
			dst[n] = r.timing
		case r != nil && ended-f.next < frameTimingCount-1:
			// The GPU has not completed the frame yet, and its record stays in the ring.
			return n, ended
		default:
			// The ring reuses the record soon, or holds the frame no more: the frame goes out
			// unfinished. A driver adds the GPU times only when the frame completes, so they
			// stay at zero.
			dst[n] = FrameTiming{Frame: f.next}
			if r != nil {
				dst[n] = r.timing
			}
		}
		f.next++
	}
	return n, ended
}
