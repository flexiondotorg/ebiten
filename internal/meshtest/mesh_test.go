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

// Package meshtest_test tests the mesh draws with instances and the depth test. The tests run each step
// in its own frame, as the depth buffer clears once a frame.
package meshtest_test

import (
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// gameUpdateCh carries steps. A step runs in the Update of each frame until it reports that it is done.
var gameUpdateCh = make(chan func() (done bool))

// runOnGameUpdate runs f in an Update of its own frame.
func runOnGameUpdate(f func()) {
	runOnGameUpdates(1, func(int) { f() })
}

// runOnGameUpdates runs f in an Update of each of n frames in a row, with the index of the frame.
//
// The caller stays blocked until the last frame ends. A caller that wakes between frames runs
// alongside the next Update, and testing.AllocsPerRun counts its allocations too.
func runOnGameUpdates(n int, f func(i int)) {
	ch := make(chan struct{})
	var i int
	gameUpdateCh <- func() bool {
		f(i)
		i++
		if i < n {
			return false
		}
		close(ch)
		return true
	}
	<-ch
}

type game struct {
	endCh chan struct{}

	// step is the step that runs in the next frame.
	step func() bool

	// waitDraw is true from a step until the Draw of its frame. A frame can run several Updates
	// to catch up with the clock, and the next step must not run in the same frame.
	waitDraw bool
}

func (g *game) Update() error {
	if g.waitDraw {
		return nil
	}
	if g.step == nil {
		select {
		case g.step = <-gameUpdateCh:
		case <-g.endCh:
			return ebiten.Termination
		}
	}
	if g.step() {
		g.step = nil
	}
	g.waitDraw = true
	return nil
}

func (g *game) Draw(*ebiten.Image) {
	g.waitDraw = false
}

func (*game) Layout(int, int) (int, int) {
	return 320, 240
}

func TestMain(m *testing.M) {
	codeCh := make(chan int)
	endCh := make(chan struct{})
	go func() {
		code := m.Run()
		close(endCh)
		codeCh <- code
	}()

	// One Update in each frame, as in the game. With a fixed TPS, a display faster than the TPS runs
	// frames without an Update. Each frame flushes into the next of the command queues, so the queue
	// that a step draws into would be random, and a queue could first grow its buffers in a late frame.
	ebiten.SetTPS(ebiten.SyncWithFPS)
	if err := ebiten.RunGame(&game{endCh: endCh}); err != nil {
		panic(err)
	}
	if code := <-codeCh; code != 0 {
		os.Exit(code)
	}
}

// meshShaderSource moves each copy of the mesh by the instance's DstX and DstY, puts it at the depth
// of the instance's Custom0, and colors it with the instance's color.
const meshShaderSource = `//kage:unit pixels

package main

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4, iDstPos vec2, iSrcPos vec2, iColor vec4, iCustom vec4) (vec4, vec2, vec4, vec4) {
	return imageDstProjection() * vec4(dstPos+iDstPos, iCustom.x, 1), srcPos, iColor, custom
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	return color
}
`

// depthShaderSource writes the depth of source 0 in 24 bits: the high byte in red, the middle byte in green, and the
// low byte in blue.
const depthShaderSource = `//kage:unit pixels

package main

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	v := floor(imageSrc0UnsafeAt(srcPos).r * 16777215)
	r := floor(v / 65536)
	g := floor((v - r*65536) / 256)
	return vec4(r, g, v-r*65536-g*256, 255) / 255
}
`

// slotsShaderSource writes the red and blue of source 0 and the depth of source 1 into red, green, and blue.
const slotsShaderSource = `//kage:unit pixels

package main

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	c := imageSrc0UnsafeAt(srcPos)
	return vec4(c.r, c.b, imageSrc1UnsafeAt(srcPos).r, 1)
}
`

// clipZShaderSource puts each vertex at z/w = Custom0, with w = 2.
const clipZShaderSource = `//kage:unit pixels

package main

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4) (vec4, vec2, vec4, vec4) {
	p := imageDstProjection() * vec4(dstPos, 0, 1)
	return vec4(p.xy*2, custom.x*2, 2), srcPos, color, custom
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	return color
}
`

// The depth of clip-space z/w runs from 0 at z/w = 0 to 1 at z/w = 1 on every graphics library.
const (
	meshSize = 16
	nearZ    = 0.25
	farZ     = 0.75
)

var (
	red   = color.RGBA{R: 0xff, A: 0xff}
	green = color.RGBA{G: 0xff, A: 0xff}
	blue  = color.RGBA{B: 0xff, A: 0xff}
)

// newQuadMesh returns a square mesh from (0, 0) to (meshSize, meshSize).
func newQuadMesh() *ebiten.Mesh {
	vs := []ebiten.Vertex{
		{DstX: 0, DstY: 0},
		{DstX: meshSize, DstY: 0},
		{DstX: 0, DstY: meshSize},
		{DstX: meshSize, DstY: meshSize},
	}
	return ebiten.NewMesh(vs, []uint32{0, 1, 2, 1, 2, 3})
}

func instance(x, y float32, z float32, clr color.RGBA) ebiten.Vertex {
	return ebiten.Vertex{
		DstX:    x,
		DstY:    y,
		ColorR:  float32(clr.R) / 0xff,
		ColorG:  float32(clr.G) / 0xff,
		ColorB:  float32(clr.B) / 0xff,
		ColorA:  float32(clr.A) / 0xff,
		Custom0: z,
	}
}

// checkPixels checks the pixels of dst against want, which gives the color at a point.
func checkPixels(t *testing.T, dst *ebiten.Image, want func(p image.Point) color.RGBA) {
	t.Helper()
	b := dst.Bounds()
	for j := b.Min.Y; j < b.Max.Y; j++ {
		for i := b.Min.X; i < b.Max.X; i++ {
			if got, want := dst.At(i, j).(color.RGBA), want(image.Pt(i, j)); got != want {
				t.Errorf("dst.At(%d, %d): got: %v, want: %v", i, j, got, want)
			}
		}
	}
}

func inQuad(p image.Point, x, y int) bool {
	return p.In(image.Rect(x, y, x+meshSize, y+meshSize))
}

func TestMeshInstances(t *testing.T) {
	runOnGameUpdate(func() {
		s, err := ebiten.NewShader([]byte(meshShaderSource))
		if err != nil {
			t.Error(err)
			return
		}
		dst := ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})

		op := &ebiten.DrawTrianglesShaderOptions{
			Mesh: newQuadMesh(),
		}
		dst.DrawTrianglesShader32([]ebiten.Vertex{
			instance(4, 8, 0, red),
			instance(40, 36, 0, blue),
		}, nil, s, op)

		checkPixels(t, dst, func(p image.Point) color.RGBA {
			switch {
			case inQuad(p, 4, 8):
				return red
			case inQuad(p, 40, 36):
				return blue
			}
			return color.RGBA{}
		})
	})
}

// TestMeshBetweenTriangles checks that a mesh draw between two triangle draws keeps the vertices of the second.
func TestMeshBetweenTriangles(t *testing.T) {
	runOnGameUpdate(func() {
		s, err := ebiten.NewShader([]byte(meshShaderSource))
		if err != nil {
			t.Error(err)
			return
		}
		dst := ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
		src := ebiten.NewImage(1, 1)
		src.Fill(color.White)

		rect := func(x, y float32, clr color.RGBA) {
			r, g, b := float32(clr.R)/0xff, float32(clr.G)/0xff, float32(clr.B)/0xff
			vs := []ebiten.Vertex{
				{DstX: x, DstY: y, ColorR: r, ColorG: g, ColorB: b, ColorA: 1},
				{DstX: x + meshSize, DstY: y, SrcX: 1, ColorR: r, ColorG: g, ColorB: b, ColorA: 1},
				{DstX: x, DstY: y + meshSize, SrcY: 1, ColorR: r, ColorG: g, ColorB: b, ColorA: 1},
				{DstX: x + meshSize, DstY: y + meshSize, SrcX: 1, SrcY: 1, ColorR: r, ColorG: g, ColorB: b, ColorA: 1},
			}
			dst.DrawTriangles(vs, []uint16{0, 1, 2, 1, 2, 3}, src, nil)
		}

		rect(0, 0, red)
		dst.DrawTrianglesShader32([]ebiten.Vertex{instance(24, 24, 0, green)}, nil, s, &ebiten.DrawTrianglesShaderOptions{
			Mesh: newQuadMesh(),
		})
		rect(48, 48, blue)

		checkPixels(t, dst, func(p image.Point) color.RGBA {
			switch {
			case inQuad(p, 0, 0):
				return red
			case inQuad(p, 24, 24):
				return green
			case inQuad(p, 48, 48):
				return blue
			}
			return color.RGBA{}
		})
	})
}

func TestMeshDepth(t *testing.T) {
	var s *ebiten.Shader
	var m *ebiten.Mesh
	var dst *ebiten.Image
	draw := func(x, y float32, z float32, clr color.RGBA, depth bool) {
		dst.DrawTrianglesShader32([]ebiten.Vertex{instance(x, y, z, clr)}, nil, s, &ebiten.DrawTrianglesShaderOptions{
			Mesh:  m,
			Depth: depth,
		})
	}

	// A far red quad, a near blue quad, and a far green quad give blue where blue and green overlap.
	runOnGameUpdate(func() {
		var err error
		s, err = ebiten.NewShader([]byte(meshShaderSource))
		if err != nil {
			t.Error(err)
			return
		}
		m = newQuadMesh()
		dst = ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})

		draw(8, 8, farZ, red, true)
		draw(16, 16, nearZ, blue, true)
		draw(24, 24, farZ, green, true)

		checkPixels(t, dst, func(p image.Point) color.RGBA {
			switch {
			case inQuad(p, 16, 16):
				return blue
			case inQuad(p, 24, 24):
				return green
			case inQuad(p, 8, 8):
				return red
			}
			return color.RGBA{}
		})
	})
	if t.Failed() {
		return
	}

	// In the next frame, the depth buffer is clear, so a far green quad covers the place of the near blue one.
	// A draw on a sub-image sets the scissor rectangle to a part of dst first, and the clear must ignore it.
	runOnGameUpdate(func() {
		dst.Clear()
		dst.SubImage(image.Rect(60, 60, 64, 64)).(*ebiten.Image).Fill(color.White)
		draw(16, 16, farZ, green, true)

		checkPixels(t, dst, func(p image.Point) color.RGBA {
			switch {
			case inQuad(p, 16, 16):
				return green
			case p.In(image.Rect(60, 60, 64, 64)):
				return color.RGBA{0xff, 0xff, 0xff, 0xff}
			}
			return color.RGBA{}
		})
	})

	// Without Depth, a far green quad covers a near blue one.
	runOnGameUpdate(func() {
		dst.Clear()
		draw(16, 16, nearZ, blue, true)
		draw(16, 16, farZ, green, false)

		checkPixels(t, dst, func(p image.Point) color.RGBA {
			if inQuad(p, 16, 16) {
				return green
			}
			return color.RGBA{}
		})
	})
}

// drawDepth draws the depth of src into dst with ImageDepth.
func drawDepth(dst, src *ebiten.Image, s *ebiten.Shader) {
	op := &ebiten.DrawTrianglesShaderOptions{
		Blend: ebiten.BlendCopy,
	}
	op.Images[0] = src
	op.ImageDepth[0] = true
	drawFull(dst, s, op)
}

// drawFull draws a quad over all of dst with op.
func drawFull(dst *ebiten.Image, s *ebiten.Shader, op *ebiten.DrawTrianglesShaderOptions) {
	w, h := float32(dst.Bounds().Dx()), float32(dst.Bounds().Dy())
	vs := []ebiten.Vertex{
		{DstX: 0, DstY: 0, SrcX: 0, SrcY: 0, ColorA: 1},
		{DstX: w, DstY: 0, SrcX: w, SrcY: 0, ColorA: 1},
		{DstX: 0, DstY: h, SrcX: 0, SrcY: h, ColorA: 1},
		{DstX: w, DstY: h, SrcX: w, SrcY: h, ColorA: 1},
	}
	dst.DrawTrianglesShader32(vs, []uint32{0, 1, 2, 1, 2, 3}, s, op)
}

// checkDepth checks the depth that depthShaderSource writes into dst against the depth that want gives at a point,
// within 2^-16.
func checkDepth(t *testing.T, dst *ebiten.Image, want func(p image.Point) float64) {
	t.Helper()
	b := dst.Bounds()
	for j := b.Min.Y; j < b.Max.Y; j++ {
		for i := b.Min.X; i < b.Max.X; i++ {
			c := dst.At(i, j).(color.RGBA)
			got := float64(int(c.R)<<16|int(c.G)<<8|int(c.B)) / (1<<24 - 1)
			w := want(image.Pt(i, j))
			if d := got - w; d < -1.0/(1<<16) || d > 1.0/(1<<16) || c.A != 0xff {
				t.Errorf("dst.At(%d, %d): got: depth %.6f (%v), want: depth %.6f", i, j, got, c, w)
			}
		}
	}
}

// newShaders compiles each source, or reports an error and returns false.
func newShaders(t *testing.T, srcs ...string) ([]*ebiten.Shader, bool) {
	t.Helper()
	ss := make([]*ebiten.Shader, len(srcs))
	for i, src := range srcs {
		s, err := ebiten.NewShader([]byte(src))
		if err != nil {
			t.Error(err)
			return nil, false
		}
		ss[i] = s
	}
	return ss, true
}

// TestMeshSourceDepth checks that a draw with SourceDepth reads the depth of two overlapping meshes, where the nearer
// mesh wins, and the clear depth elsewhere.
func TestMeshSourceDepth(t *testing.T) {
	var supported bool
	runOnGameUpdate(func() {
		if supported = ebiten.IsDepthSourceSupported(); !supported {
			return
		}
		s, err := ebiten.NewShader([]byte(meshShaderSource))
		if err != nil {
			t.Error(err)
			return
		}
		ds, err := ebiten.NewShader([]byte(depthShaderSource))
		if err != nil {
			t.Error(err)
			return
		}
		m := newQuadMesh()
		a := ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
		b := ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})

		// The near quad draws first, so that the far quad passes the depth test only where it does not overlap.
		for _, i := range []ebiten.Vertex{instance(16, 16, nearZ, blue), instance(8, 8, farZ, red)} {
			a.DrawTrianglesShader32([]ebiten.Vertex{i}, nil, s, &ebiten.DrawTrianglesShaderOptions{
				Mesh:  m,
				Depth: true,
			})
		}
		drawDepth(b, a, ds)

		checkDepth(t, b, func(p image.Point) float64 {
			switch {
			case inQuad(p, 16, 16):
				return nearZ
			case inQuad(p, 8, 8):
				return farZ
			}
			return 1
		})
	})
	if !supported {
		t.Skip("the graphics driver cannot read a depth buffer")
	}
}

// TestMeshDepthReadOnly checks that a DepthReadOnly mesh at 0.5 draws only where the depth is farther, over the far
// quad and the clear depth but not over the near quad, and leaves the depth as it was.
func TestMeshDepthReadOnly(t *testing.T) {
	runOnGameUpdate(func() {
		if !ebiten.IsDepthSourceSupported() {
			return
		}
		ss, ok := newShaders(t, meshShaderSource, depthShaderSource)
		if !ok {
			return
		}
		m := newQuadMesh()
		a := ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
		b := ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})

		for _, i := range []ebiten.Vertex{instance(8, 8, nearZ, blue), instance(24, 24, farZ, red)} {
			a.DrawTrianglesShader32([]ebiten.Vertex{i}, nil, ss[0], &ebiten.DrawTrianglesShaderOptions{
				Mesh:  m,
				Depth: true,
			})
		}
		a.DrawTrianglesShader32([]ebiten.Vertex{instance(16, 16, 0.5, green)}, nil, ss[0], &ebiten.DrawTrianglesShaderOptions{
			Mesh:          m,
			Depth:         true,
			DepthReadOnly: true,
		})
		drawDepth(b, a, ss[1])

		checkPixels(t, a, func(p image.Point) color.RGBA {
			switch {
			case inQuad(p, 8, 8):
				return blue
			case inQuad(p, 16, 16):
				return green
			case inQuad(p, 24, 24):
				return red
			}
			return color.RGBA{}
		})
		checkDepth(t, b, func(p image.Point) float64 {
			switch {
			case inQuad(p, 8, 8):
				return nearZ
			case inQuad(p, 24, 24):
				return farZ
			}
			return 1
		})
	})
}

// TestImageDepthSlots checks that one image can be the color of source 0 and the depth of source 1 of one draw.
func TestImageDepthSlots(t *testing.T) {
	runOnGameUpdate(func() {
		if !ebiten.IsDepthSourceSupported() {
			return
		}
		ss, ok := newShaders(t, meshShaderSource, slotsShaderSource)
		if !ok {
			return
		}
		m := newQuadMesh()
		a := ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
		b := ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
		for _, i := range []ebiten.Vertex{instance(8, 8, nearZ, blue), instance(24, 24, farZ, red)} {
			a.DrawTrianglesShader32([]ebiten.Vertex{i}, nil, ss[0], &ebiten.DrawTrianglesShaderOptions{
				Mesh:  m,
				Depth: true,
			})
		}
		op := &ebiten.DrawTrianglesShaderOptions{Blend: ebiten.BlendCopy}
		op.Images[0] = a
		op.Images[1] = a
		op.ImageDepth[1] = true
		drawFull(b, ss[1], op)

		checkPixels(t, b, func(p image.Point) color.RGBA {
			switch {
			case inQuad(p, 8, 8):
				return color.RGBA{G: 0xff, B: 0x40, A: 0xff}
			case inQuad(p, 24, 24):
				return color.RGBA{R: 0xff, B: 0xbf, A: 0xff}
			}
			return color.RGBA{B: 0xff, A: 0xff}
		})
	})
}

// TestImageDepthSelfRead checks that a draw reads the depth of its own destination after a depth write in the same
// frame, and again after a later depth write.
func TestImageDepthSelfRead(t *testing.T) {
	runOnGameUpdate(func() {
		if !ebiten.IsDepthSourceSupported() {
			return
		}
		ss, ok := newShaders(t, meshShaderSource, depthShaderSource)
		if !ok {
			return
		}
		m := newQuadMesh()
		a := ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
		draw := func(i ebiten.Vertex) {
			a.DrawTrianglesShader32([]ebiten.Vertex{i}, nil, ss[0], &ebiten.DrawTrianglesShaderOptions{
				Mesh:  m,
				Depth: true,
			})
		}

		draw(instance(8, 8, nearZ, blue))
		drawDepth(a, a, ss[1])
		draw(instance(40, 40, farZ, red))
		drawDepth(a, a, ss[1])

		checkDepth(t, a, func(p image.Point) float64 {
			switch {
			case inQuad(p, 8, 8):
				return nearZ
			case inQuad(p, 40, 40):
				return farZ
			}
			return 1
		})
	})
}

// TestDepthClipZ checks that a triangle draw at clip-space z/w of 0, 0.25, 0.75, and 1 reads back the depth 0, 0.25,
// 0.75, and 1.
func TestDepthClipZ(t *testing.T) {
	runOnGameUpdate(func() {
		if !ebiten.IsDepthSourceSupported() {
			return
		}
		ss, ok := newShaders(t, clipZShaderSource, depthShaderSource)
		if !ok {
			return
		}
		a := ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
		b := ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
		zs := []float32{0, nearZ, farZ, 1}
		var vs []ebiten.Vertex
		var is []uint32
		for k, z := range zs {
			x := float32(k * meshSize)
			n := uint32(len(vs))
			for _, c := range [][2]float32{{0, 0}, {meshSize, 0}, {0, meshSize}, {meshSize, meshSize}} {
				vs = append(vs, ebiten.Vertex{DstX: x + c[0], DstY: c[1], ColorG: 1, ColorA: 1, Custom0: z})
			}
			is = append(is, n, n+1, n+2, n+1, n+2, n+3)
		}
		a.DrawTrianglesShader32(vs, is, ss[0], &ebiten.DrawTrianglesShaderOptions{Depth: true})
		drawDepth(b, a, ss[1])

		checkDepth(t, b, func(p image.Point) float64 {
			if p.Y < meshSize {
				return float64(zs[p.X/meshSize])
			}
			return 1
		})
	})
}

// TestMeshDepthDiscard checks that the depth discard at the end of a frame keeps the pixels, and that it waits for the
// last depth draw to a destination when the frame draws to another destination between depth draws, and for a read of
// the depth after the last depth draw. The pixels are read in the next frame, because a read flushes the frame in the
// middle, and only the last flush of a frame discards.
func TestMeshDepthDiscard(t *testing.T) {
	var s, ds *ebiten.Shader
	var m *ebiten.Mesh
	var dst, other, depth *ebiten.Image
	draw := func(img *ebiten.Image, x, y float32, z float32, clr color.RGBA) {
		img.DrawTrianglesShader32([]ebiten.Vertex{instance(x, y, z, clr)}, nil, s, &ebiten.DrawTrianglesShaderOptions{
			Mesh:  m,
			Depth: true,
		})
	}

	runOnGameUpdates(2, func(i int) {
		if i == 0 {
			var err error
			s, err = ebiten.NewShader([]byte(meshShaderSource))
			if err != nil {
				t.Error(err)
				return
			}
			m = newQuadMesh()
			dst = ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
			other = ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})

			// A near blue quad, a draw to another image, and a far green quad that the depth of the blue quad hides.
			draw(dst, 16, 16, nearZ, blue)
			draw(other, 0, 0, nearZ, red)
			draw(dst, 24, 24, farZ, green)

			// A read of the depth of dst after its last depth draw, and a draw that binds another image.
			if ebiten.IsDepthSourceSupported() {
				ds, err = ebiten.NewShader([]byte(depthShaderSource))
				if err != nil {
					t.Error(err)
					return
				}
				depth = ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
				drawDepth(depth, dst, ds)
				draw(other, 0, 0, nearZ, red)
			}
			return
		}
		if s == nil {
			return
		}
		if depth != nil {
			checkDepth(t, depth, func(p image.Point) float64 {
				switch {
				case inQuad(p, 16, 16):
					return nearZ
				case inQuad(p, 24, 24):
					return farZ
				}
				return 1
			})
		}
		checkPixels(t, dst, func(p image.Point) color.RGBA {
			switch {
			case inQuad(p, 16, 16):
				return blue
			case inQuad(p, 24, 24):
				return green
			}
			return color.RGBA{}
		})
	})
}

// sumShaderSource adds the colors of sources 0 and 1.
const sumShaderSource = `//kage:unit pixels

package main

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	return imageSrc0UnsafeAt(srcPos) + imageSrc1UnsafeAtFromSrc0Pos(srcPos)
}
`

// TestEmptySourceSlot checks that a slot without an image reads transparent black, after an earlier draw of the same
// shader had the destination of this draw in that slot.
func TestEmptySourceSlot(t *testing.T) {
	runOnGameUpdate(func() {
		ss, ok := newShaders(t, sumShaderSource)
		if !ok {
			return
		}
		newImage := func(clr color.Color) *ebiten.Image {
			img := ebiten.NewImageWithOptions(image.Rect(0, 0, meshSize, meshSize), &ebiten.NewImageOptions{Unmanaged: true})
			img.Fill(clr)
			return img
		}
		a, b, c := newImage(green), newImage(red), newImage(color.Transparent)

		op := &ebiten.DrawTrianglesShaderOptions{Blend: ebiten.BlendCopy}
		op.Images[0] = a
		op.Images[1] = b
		drawFull(c, ss[0], op)
		op.Images[1] = nil
		drawFull(b, ss[0], op)

		checkPixels(t, c, func(image.Point) color.RGBA { return color.RGBA{R: 0xff, G: 0xff, A: 0xff} })
		checkPixels(t, b, func(image.Point) color.RGBA { return green })
	})
}

// TestMeshAllocations checks that mesh draws with instances and Depth allocate nothing after the first frames.
func TestMeshAllocations(t *testing.T) {
	var s *ebiten.Shader
	var m *ebiten.Mesh
	var dst *ebiten.Image
	instances := []ebiten.Vertex{
		instance(8, 8, farZ, red),
		instance(16, 16, nearZ, blue),
		instance(24, 24, farZ, green),
	}
	op := &ebiten.DrawTrianglesShaderOptions{
		Depth: true,
	}
	draw := func() {
		for range 8 {
			dst.DrawTrianglesShader32(instances, nil, s, op)
		}
	}

	runOnGameUpdate(func() {
		var err error
		s, err = ebiten.NewShader([]byte(meshShaderSource))
		if err != nil {
			t.Error(err)
			return
		}
		m = newQuadMesh()
		op.Mesh = m
		dst = ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
	})
	if t.Failed() {
		return
	}

	// The first frames grow the pools and the buffers of each command queue. The frames run in one
	// step, so that this goroutine stays blocked while they measure.
	allocs := make([]float64, 10)
	runOnGameUpdates(len(allocs), func(i int) {
		allocs[i] = testing.AllocsPerRun(1, draw)
	})
	for i, a := range allocs {
		if i >= 5 && a != 0 {
			t.Errorf("frame %d: allocations: got: %v, want: 0", i, a)
		}
	}
}
