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
	"testing"
	"time"
)

// runFrame times frame as the driver does: a start, two passes,
// a queue wait, and its end. When complete is true, the frame gets one command buffer of 2 ms
// and finishes.
func runFrame(f *FrameTimings, frame int64, complete bool) {
	f.Begin(frame, float64(frame))
	f.AddPass(frame, false)
	f.AddPass(frame, true)
	f.AddQueueWait(frame, time.Millisecond)
	f.End(frame)
	if complete {
		completeFrame(f, frame)
	}
}

func completeFrame(f *FrameTimings, frame int64) {
	f.AddGPU(frame, float64(frame)+0.001, float64(frame)+0.003)
	f.Finish(frame)
}

func TestFrameTimingsOrder(t *testing.T) {
	var f FrameTimings
	var dst [8]FrameTiming
	if n, frame := f.Read(dst[:]); n != 0 || frame != -1 {
		t.Fatalf("first read: got %d records and frame %d, want 0 and -1", n, frame)
	}
	// Frame 0 runs while the timing turns on, so it is not read.
	runFrame(&f, 0, true)
	for frame := int64(1); frame <= 3; frame++ {
		runFrame(&f, frame, frame != 2)
	}
	n, frame := f.Read(dst[:])
	if frame != 3 {
		t.Errorf("latest frame: got %d, want 3", frame)
	}
	// Frame 2 is not complete, so frame 3 waits for it.
	if n != 1 || dst[0].Frame != 1 {
		t.Fatalf("got %d records %+v, want frame 1 only", n, dst[:n])
	}
	want := FrameTiming{Frame: 1, Start: 1, GPUStart: 1.001, GPUEnd: 1.003, GPU: 2 * time.Millisecond, QueueWait: time.Millisecond, Passes: 2, DepthPasses: 1}
	if got := dst[0]; got.Frame != want.Frame || got.Start != want.Start || got.GPUStart != want.GPUStart || got.GPUEnd != want.GPUEnd ||
		(got.GPU-want.GPU).Abs() > time.Microsecond || got.QueueWait != want.QueueWait || got.Passes != want.Passes || got.DepthPasses != want.DepthPasses {
		t.Errorf("frame 1: got %+v, want %+v", got, want)
	}
	completeFrame(&f, 2)
	if n, _ := f.Read(dst[:]); n != 2 || dst[0].Frame != 2 || dst[1].Frame != 3 {
		t.Errorf("got %d records %+v, want frames 2 and 3", n, dst[:n])
	}
	if n, _ := f.Read(dst[:]); n != 0 {
		t.Errorf("got %d records on a read with no new frame, want 0", n)
	}
}

func TestFrameTimingsShortDestination(t *testing.T) {
	var f FrameTimings
	var dst [2]FrameTiming
	f.Read(nil)
	for frame := int64(0); frame <= 5; frame++ {
		runFrame(&f, frame, true)
	}
	var frames []int64
	for range 3 {
		n, _ := f.Read(dst[:])
		for _, t := range dst[:n] {
			frames = append(frames, t.Frame)
		}
	}
	if len(frames) != 5 || frames[0] != 1 || frames[4] != 5 {
		t.Errorf("got frames %v, want 1 to 5", frames)
	}
}

func TestFrameTimingsWrap(t *testing.T) {
	var f FrameTimings
	var dst [frameTimingCount * 2]FrameTiming
	f.Read(nil)
	runFrame(&f, 0, true)
	// Frame 1 never completes. When the ring is about to reuse its record, it goes out unfinished.
	runFrame(&f, 1, false)
	last := int64(frameTimingCount)
	for frame := int64(2); frame < last; frame++ {
		runFrame(&f, frame, true)
	}
	if n, _ := f.Read(dst[:]); n != 0 {
		t.Fatalf("got %d records before the wrap, want 0", n)
	}
	runFrame(&f, last, true)
	n, _ := f.Read(dst[:])
	if n != int(last) {
		t.Fatalf("got %d records, want %d", n, last)
	}
	if got := dst[0]; got.Frame != 1 || got.GPU != 0 || got.GPUEnd != 0 || got.Passes != 2 {
		t.Errorf("unfinished frame: got %+v, want frame 1 with 2 passes and no GPU time", got)
	}
	// The next frames reuse the ring from its start.
	for frame := last + 1; frame < last+frameTimingCount+10; frame++ {
		runFrame(&f, frame, true)
	}
	n, frame := f.Read(dst[:])
	if n != frameTimingCount+9 || dst[0].Frame != last+1 || dst[n-1].Frame != frame {
		t.Errorf("after the wrap: got %d records from %d to %d, latest frame %d", n, dst[0].Frame, dst[n-1].Frame, frame)
	}
}

func TestFrameTimingsLateCompletion(t *testing.T) {
	var f FrameTimings
	var dst [frameTimingCount * 2]FrameTiming
	f.Read(nil)
	for frame := int64(0); frame < 2*frameTimingCount; frame++ {
		runFrame(&f, frame, frame != 1)
	}
	f.Read(dst[:])
	// The GPU completes frame 1 after the ring reuses its record: the record of frame 65 stays.
	completeFrame(&f, 1)
	r := f.find(1 + frameTimingCount)
	if r == nil || (r.timing.GPU-2*time.Millisecond).Abs() > time.Microsecond {
		t.Errorf("record of frame %d: got %+v, want its own GPU time only", 1+frameTimingCount, r)
	}
}

func TestFrameTimingsAllocations(t *testing.T) {
	var f FrameTimings
	var dst [4]FrameTiming
	f.Read(nil)
	var frame int64
	allocs := testing.AllocsPerRun(100, func() {
		runFrame(&f, frame, true)
		frame++
		f.Read(dst[:])
	})
	if allocs != 0 {
		t.Errorf("got %v allocations a frame, want 0", allocs)
	}
}
