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

package ebiten

import (
	"time"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
	"github.com/hajimehoshi/ebiten/v2/internal/ui"
)

// FrameTiming is the GPU timing of one frame of the graphics driver.
type FrameTiming struct {
	// Frame is the frame number of the graphics driver.
	Frame int64

	// Start is the host time of the frame start in seconds, on the clock of GPUStart and GPUEnd.
	// It is zero on OpenGL.
	Start float64

	// GPUStart and GPUEnd are the host times in seconds when the GPU starts the first command
	// buffer of the frame and ends the last one. They are zero on OpenGL.
	GPUStart, GPUEnd float64

	// GPU is the sum of the GPU times of the command buffers of the frame on Metal, and of the
	// timer queries of the flushes of the frame on OpenGL. It is zero when the GPU time is unknown.
	GPU time.Duration

	// QueueWait is the time that the frame waits for new command buffers from the queue.
	QueueWait time.Duration

	// Passes is the count of render passes of the frame.
	Passes int
}

// The conversion compiles only while the two types have the same fields, so that
// ReadFrameTimings can read into dst without a copy.
var _ = graphicsdriver.FrameTiming(FrameTiming{})

// ReadFrameTimings copies the GPU timing of the frames that the GPU finished since the last call
// into dst, in frame order, and returns their count and the number of the latest ended frame.
// The frame number is negative when the graphics driver reports no timing (DirectX, WebGL, and
// OpenGL without timer queries), or before the first frame ends.
//
// The driver times the frames only after the first call, which returns no records. Call it at
// the start of Update: frame is then the frame that the last Draw filled, and its record arrives
// in a later call. A record that waits for more than 64 frames goes out with its missing values
// at zero. ReadFrameTimings makes no allocation. Metal and OpenGL report timing today.
func ReadFrameTimings(dst []FrameTiming) (n int, frame int64) {
	d := unsafe.Slice((*graphicsdriver.FrameTiming)(unsafe.Pointer(unsafe.SliceData(dst))), len(dst))
	return ui.Get().ReadFrameTimings(d)
}
