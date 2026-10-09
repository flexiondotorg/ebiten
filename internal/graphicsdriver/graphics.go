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

// MeshID is the ID of a mesh.
type MeshID int

// MeshDrawer draws a GPU-resident mesh once for each instance record. A driver implements it optionally.
type MeshDrawer interface {
	CanDrawMesh() bool

	// NewMesh uploads the vertices and the indices under the ID.
	NewMesh(id MeshID, vertices []float32, indices []uint32) error

	// DrawMesh uploads the instance records, gives dst a depth buffer on its first depth draw and
	// clears it once a frame, and draws. An unknown mesh ID draws nothing.
	DrawMesh(dst ImageID, srcs [graphics.ShaderSrcImageCount]ImageID, shader ShaderID, mesh MeshID, instances []float32, blend Blend, uniforms []uint32, depth bool) error
}

// DepthSourcer binds the depth buffer of source 0 for a draw with Blend.SourceDepth. A driver implements it optionally.
type DepthSourcer interface {
	CanReadDepth() bool
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

	// Passes is the count of render passes of the frame, and DepthPasses the count of the passes
	// with a depth attachment.
	Passes, DepthPasses int
}

// FrameTimer reports the GPU timing of each frame. A driver implements it optionally.
type FrameTimer interface {
	// ReadFrameTimings copies the finished frame records in frame order into dst, and returns
	// their count and the number of the latest ended frame, or -1 before the first frame ends.
	// A record that waits for more frames than the driver keeps goes out with its missing
	// values at zero. The driver times the frames only after the first call.
	ReadFrameTimings(dst []FrameTiming) (n int, frame int64)
}

// PassTime is the GPU time of one render pass.
type PassTime struct {
	// Dst is the destination image of the pass, and Width and Height are its size.
	Dst           ImageID
	Width, Height int

	// Depth reports whether the pass has a depth attachment.
	Depth bool

	// GPU is the GPU time of the command buffer of the pass on Metal, and of a timer query on
	// OpenGL.
	GPU time.Duration
}

// PassTimer times each render pass of one frame. A driver implements it optionally. Only the
// render thread calls it.
type PassTimer interface {
	// TimePasses times the render passes of the frame that the next Begin starts, until the End
	// of the frame, and reports false when the driver cannot time them. On Metal, each timed pass
	// runs in its own command buffer, so the frame takes longer.
	TimePasses() bool

	// ReadPassTimes appends the pass times of the timed frame to dst in pass order, and reports
	// false while the GPU has not completed the frame.
	ReadPassTimes(dst []PassTime) ([]PassTime, bool)
}

// RendererNamer names the renderer of the graphics library. A driver implements it optionally.
type RendererNamer interface {
	// RendererName returns the renderer, the vendor, and the version that the graphics library
	// reports, or empty strings before the context exists or when the library reports none.
	RendererName() (renderer, vendor, version string)
}
