// Copyright 2018 The Ebiten Authors
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
	"image"
	"time"

	"github.com/hajimehoshi/ebiten/v2/internal/color"
	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/shaderir"
)

type DstRegion struct {
	Region     image.Rectangle
	IndexCount int
}

const (
	InvalidImageID  = 0
	InvalidShaderID = 0
)

// FlushMode specifies whether a command batch completes or presents a frame.
type FlushMode int

const (
	// FlushModeIntermediate submits commands without completing the frame.
	FlushModeIntermediate FlushMode = iota
	// FlushModeEndFrame completes the frame without presenting it.
	FlushModeEndFrame
	// FlushModePresent completes and presents the frame.
	FlushModePresent
)

type Graphics interface {
	Initialize() error
	ColorSpace() color.ColorSpace
	Begin() error
	// End ends a command batch with the given flush mode.
	End(mode FlushMode) error
	SetTransparent(transparent bool)
	SetVertices(vertices []float32, indices []uint32) error
	NewImage(width, height int) (Image, error)
	NewScreenFramebufferImage(width, height int) (Image, error)
	SetVsyncEnabled(enabled bool)
	NeedsClearingScreen() bool
	MaxImageSize() int

	NewShader(program *shaderir.Program) (Shader, error)

	// DrawTriangles draws an image onto another image with the given parameters.
	DrawTriangles(dst ImageID, srcs [graphics.ShaderSrcImageCount]ImageID, shader ShaderID, dstRegions []DstRegion, indexOffset int, blend Blend, uniforms []uint32) error
}

type Resetter interface {
	Reset() error
}

type Image interface {
	ID() ImageID
	Dispose()
	// ReadPixelsAsync starts reading pixels. Hardware drivers must return without waiting for the GPU.
	//
	// The read pixels include the drawing commands preceding this call and exclude the following
	// ones, as the reads are recorded at the current position of the command stream.
	//
	// ReadPixelsAsync must be called on the render thread.
	//
	// The returned read-back must remain valid after the image is disposed.
	ReadPixelsAsync(args []PixelsArgs) (PixelReadback, error)
	WritePixels(args []PixelsArgs) error
}

// PixelReadback represents a pending pixel read-back started by Image.ReadPixelsAsync.
type PixelReadback interface {
	// Poll reports whether the read-back has completed. Poll must not wait for the GPU.
	//
	// When Poll reports true without an error, the read pixels have been copied into the
	// arguments passed to ReadPixelsAsync. When Poll returns an error, the read-back has failed
	// and their contents are unspecified. Poll must not be called again after it reports done or
	// returns an error.
	Poll() (done bool, err error)

	// Wait completes the read-back and copies its pixels, or returns an error.
	// Wait must be called on the render thread, and must not be called after
	// Poll reports completion or an error. Neither Poll nor Wait may be called after Wait.
	// For hardware drivers, Wait must be called outside a Begin/End command batch.
	Wait() error

	// Dispose releases the resources for the read-back.
	// Dispose must be called exactly once, whether the read-back completed or was aborted.
	Dispose()
}

type ImageID int

type PixelsArgs struct {
	Pixels []byte
	Region image.Rectangle
}

type Shader interface {
	ID() ShaderID
	Dispose()
}

type ShaderID int

// FrameTiming is the GPU timing of one frame of a driver.
type FrameTiming struct {
	// Frame is the frame number of the driver.
	Frame int64

	// Start is the host time of the frame start in seconds, on the clock of GPUStart and GPUEnd.
	// It is zero on OpenGL.
	Start float64

	// GPUStart and GPUEnd are the host times in seconds when the GPU starts the first command
	// buffer of the frame and ends the last one. They are zero on OpenGL.
	GPUStart, GPUEnd float64

	// GPU is the time that the GPU spent on the command buffers of the frame on Metal, without the
	// overlaps with the earlier command buffers, and the sum of the timer queries of the flushes of
	// the frame on OpenGL. It is zero when the GPU time is unknown.
	GPU time.Duration

	// QueueWait is the time that the frame waits for new command buffers from the queue.
	QueueWait time.Duration

	// Passes is the count of render passes of the frame.
	Passes int
}

// FrameTimer reports the GPU timing of each frame. A driver implements it optionally.
type FrameTimer interface {
	// ReadFrameTimings copies the finished frame records in frame order into dst, and returns
	// their count and the number of the latest ended frame, or -1 before the first frame ends.
	// A record that waits for more frames than the driver keeps goes out with its missing
	// values at zero. The driver times the frames only after the first call.
	ReadFrameTimings(dst []FrameTiming) (n int, frame int64)
}
