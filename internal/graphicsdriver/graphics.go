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
	ReadPixels(args []PixelsArgs) error
	WritePixels(args []PixelsArgs) error
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

	// GPU is the sum of the GPU times of the command buffers of the frame on Metal, and of the
	// timer queries of the flushes of the frame on OpenGL. It is zero when the GPU time is unknown.
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

// RendererNamer names the renderer of the graphics library. A driver implements it optionally.
type RendererNamer interface {
	// RendererName returns the renderer, the vendor, and the version that the graphics library
	// reports, or empty strings before the context exists or when the library reports none.
	RendererName() (renderer, vendor, version string)
}
