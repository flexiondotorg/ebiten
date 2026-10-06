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

const (
	meshSize = 16
	nearZ    = -0.5
	farZ     = 0.5
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

// TestMeshDepthDiscard checks that the depth discard at the end of a frame keeps the pixels, and that it waits for the
// last depth draw to a destination when the frame draws to another destination between depth draws. The pixels are
// read in the next frame, because a read flushes the frame in the middle, and only the last flush of a frame discards.
func TestMeshDepthDiscard(t *testing.T) {
	var s *ebiten.Shader
	var m *ebiten.Mesh
	var dst, other *ebiten.Image
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
			return
		}
		if s == nil {
			return
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
