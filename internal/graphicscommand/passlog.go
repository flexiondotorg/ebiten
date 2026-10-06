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

package graphicscommand

import (
	"bytes"
	"cmp"
	"fmt"
	"image"
	"os"
	"slices"
	"sync/atomic"
	"time"

	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
)

// The render-pass log is a diagnostic hook. RequestPassLog asks for the log of one frame, and
// ebiten.CaptureRenderPasses calls it.
//
// The log groups the draw commands of one frame into render passes as the Metal driver does: a new
// pass at each flush, after a pixel write or read, when the destination changes, and when a depth
// draw needs the first depth clear of its destination in the frame. The command stream is the same
// on every graphics driver, so a run with any driver gives the passes that Metal makes.
//
// A graphics driver that implements graphicsdriver.PassTimer (Metal and OpenGL) times each render pass
// of the captured frame, and the log waits for the GPU to complete the frame before it writes the file
// with the GPU time of each pass. Only the captured frame changes how the frame reaches the GPU.

// passLogWait is the count of frames that a captured frame waits for the GPU times of its passes.
const passLogWait = 120

// passLog is nil when the log is off, and after the log is written. Only the render thread uses it.
var passLog *passLogger

// passLogRequest holds the path of the pass log that RequestPassLog asks for, until the end of a
// frame starts it.
var passLogRequest atomic.Pointer[string]

// RequestPassLog asks for a pass log of one frame, written to path. The log starts at the end of
// the next frame while no other pass log runs, and captures its second frame, so that it knows
// which images have a depth buffer. A later request replaces a request that waits.
func RequestPassLog(path string) {
	passLogRequest.Store(&path)
}

// startRequestedPassLog starts a requested pass log. Call it at the end of a frame.
func startRequestedPassLog() {
	if passLog != nil || passLogRequest.Load() == nil {
		return
	}
	if path := passLogRequest.Swap(nil); path != nil {
		passLog = newPassLogger(*path, 1)
	}
}

func newPassLogger(path string, frame int64) *passLogger {
	return &passLogger{
		path:    path,
		capture: frame,
		depth:   map[int]bool{},
		cleared: map[int]bool{},
	}
}

type passLogger struct {
	path string

	// capture is the frame to capture, counted from 0.
	capture int64

	frame   int64
	inFrame bool

	// capturing reports whether the current frame is the captured frame.
	capturing bool

	// depth holds the IDs of the images that have a depth buffer. The driver gives an image a
	// depth buffer at its first depth draw, and the image keeps it.
	depth map[int]bool

	// cleared holds the IDs of the images whose depth buffer the captured frame cleared.
	cleared map[int]bool

	lastDst *Image
	entries []passEntry
	passes  int
	flushes int

	// timer times the passes of the captured frame, or is nil.
	timer graphicsdriver.PassTimer

	// waiting reports whether the captured frame ended and waits for its pass times, for waited
	// frames.
	waiting bool
	waited  int

	// times holds the pass times of the captured frame, when timed is true.
	times []graphicsdriver.PassTime
	timed bool
}

// passEntry is a render pass, or a line for a command or a flush outside the render passes.
type passEntry struct {
	// line is not empty for an entry that is not a render pass.
	line string

	index      int
	dst        int
	width      int
	height     int
	texWidth   int
	texHeight  int
	screen     bool
	depth      bool
	clearDepth bool
	draws      int
	meshes     int
	shaders    []int

	// The first draw of the pass.
	blend  graphicsdriver.Blend
	srcs   [graphics.ShaderSrcImageCount]int
	region image.Rectangle
	mesh   bool
}

// observe records the commands of one flush. The commands are not executed yet. driver can be nil.
func (p *passLogger) observe(commands []command, mode graphicsdriver.FlushMode, driver graphicsdriver.Graphics) {
	if p.waiting {
		p.poll(mode)
		return
	}
	if !p.inFrame {
		p.inFrame = true
		p.capturing = p.frame == p.capture
		if p.capturing {
			if t, ok := driver.(graphicsdriver.PassTimer); ok && t.TimePasses() {
				p.timer = t
			}
		}
	}

	// The graphics driver begins and ends at each flush, and that ends the render pass.
	p.lastDst = nil
	if p.capturing {
		p.flushes++
	}
	for _, c := range commands {
		p.observeCommand(c)
	}
	if p.capturing {
		var name string
		switch mode {
		case graphicsdriver.FlushModeIntermediate:
			name = "intermediate"
		case graphicsdriver.FlushModeEndFrame:
			name = "end of frame"
		case graphicsdriver.FlushModePresent:
			name = "present"
		}
		p.entries = append(p.entries, passEntry{line: fmt.Sprintf("flush %d (%s) ends the render pass", p.flushes, name)})
	}

	if mode != graphicsdriver.FlushModeIntermediate {
		p.inFrame = false
		if p.capturing {
			p.capturing = false
			if p.timer != nil {
				p.waiting = true
				return
			}
			p.done()
			return
		}
		p.frame++
	}
}

func (p *passLogger) observeCommand(c command) {
	switch c := c.(type) {
	case *drawTrianglesCommand:
		if len(c.dstRegions) == 0 {
			return
		}
		var srcs [graphics.ShaderSrcImageCount]int
		for i, src := range c.srcs {
			if src != nil {
				srcs[i] = src.id
			}
		}
		p.observeDraw(c.dst, c.shader.id, c.blend, srcs, c.dstRegions[0].Region, false, c.blend.DepthTest)
	case *drawMeshCommand:
		if len(c.instances) == 0 {
			return
		}
		var srcs [graphics.ShaderSrcImageCount]int
		for i, src := range c.srcs {
			if src != nil {
				srcs[i] = src.id
			}
		}
		p.observeDraw(c.dst, c.shader.id, c.blend, srcs, image.Rectangle{}, true, c.depth)
	case *writePixelsCommand, *readPixelsCommand:
		// The Metal driver ends the render pass to copy pixels.
		p.lastDst = nil
		if p.capturing {
			p.entries = append(p.entries, passEntry{line: c.String() + "; ends the render pass"})
		}
	default:
		if p.capturing {
			p.entries = append(p.entries, passEntry{line: c.String()})
		}
	}
}

func (p *passLogger) observeDraw(dst *Image, shader int, blend graphicsdriver.Blend, srcs [graphics.ShaderSrcImageCount]int, region image.Rectangle, mesh, depth bool) {
	if depth {
		p.depth[dst.id] = true
	}
	if !p.capturing {
		return
	}

	clearDepth := depth && !p.cleared[dst.id]
	if p.lastDst != dst || clearDepth {
		hasDepth := p.depth[dst.id]
		if hasDepth && clearDepth {
			p.cleared[dst.id] = true
		}
		p.passes++
		e := passEntry{
			index:      p.passes,
			dst:        dst.id,
			width:      dst.width,
			height:     dst.height,
			texWidth:   dst.width,
			texHeight:  dst.height,
			screen:     dst.screen,
			depth:      hasDepth,
			clearDepth: hasDepth && clearDepth,
			blend:      blend,
			srcs:       srcs,
			region:     region,
			mesh:       mesh,
		}
		if !dst.screen {
			// Do not call InternalSize, which caches the size on the image that another thread uses.
			e.texWidth = graphics.InternalImageSize(dst.width)
			e.texHeight = graphics.InternalImageSize(dst.height)
		}
		p.entries = append(p.entries, e)
	}
	p.lastDst = dst

	e := &p.entries[len(p.entries)-1]
	e.draws++
	if mesh {
		e.meshes++
	}
	if !slices.Contains(e.shaders, shader) {
		e.shaders = append(e.shaders, shader)
	}
}

// poll reads the pass times of the captured frame, and writes the log when they are ready, or
// without them after passLogWait frames.
func (p *passLogger) poll(mode graphicsdriver.FlushMode) {
	times, ok := p.timer.ReadPassTimes(p.times[:0])
	p.times = times
	if ok {
		p.timed = true
		p.done()
		return
	}
	if mode != graphicsdriver.FlushModeIntermediate {
		p.waited++
	}
	if p.waited >= passLogWait {
		p.done()
	}
}

// timesMatch reports whether the driver timed the passes of the log: the same count, and in each
// pass the same size and depth attachment.
func (p *passLogger) timesMatch() bool {
	if !p.timed {
		return false
	}
	var i int
	for _, e := range p.entries {
		if e.line != "" {
			continue
		}
		if i >= len(p.times) {
			return false
		}
		t := p.times[i]
		if t.Width != e.width || t.Height != e.height || t.Depth != e.depth {
			return false
		}
		i++
	}
	return i == len(p.times)
}

func (p *passLogger) done() {
	passLog = nil
	var buf bytes.Buffer
	p.write(&buf)
	// A reader that waits for the file never sees a part of it.
	tmp := p.path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "graphicscommand: writing the pass log: %v\n", err)
		return
	}
	if err := os.Rename(tmp, p.path); err != nil {
		fmt.Fprintf(os.Stderr, "graphicscommand: writing the pass log: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stderr, "graphicscommand: wrote the render passes of frame %d to %s\n", p.frame, p.path)
}

// passTraffic returns the estimated bytes that a tile-based GPU loads and stores for the pass: 4 bytes
// a pixel of the whole texture for colour and for depth. An offscreen pass loads and stores the colour
// image, and a screen pass clears it and stores it. A depth buffer loads unless the pass clears it, and
// stores.
func passTraffic(e *passEntry) (loaded, stored int64) {
	px := int64(e.texWidth) * int64(e.texHeight)
	stored = 4 * px
	if !e.screen {
		loaded = 4 * px
	}
	if e.depth {
		stored += 4 * px
		if !e.clearDepth {
			loaded += 4 * px
		}
	}
	return loaded, stored
}

func (p *passLogger) write(w *bytes.Buffer) {
	fmt.Fprintf(w, "Render passes of frame %d, grouped as the Metal driver groups them\n", p.frame)
	joined := p.timesMatch()
	switch {
	case joined:
		fmt.Fprint(w, "GPU time: the GPU time of each pass, which the graphics driver measures by itself (Metal: in its own command buffer; OpenGL: with a timer query)\n")
	case p.timed:
		fmt.Fprintf(w, "GPU time: the graphics driver made %d passes, which do not match the passes of the log, so they follow the summary\n", len(p.times))
	case p.timer != nil:
		fmt.Fprintf(w, "GPU time: none, as the GPU did not complete the frame in %d frames\n", passLogWait)
	default:
		fmt.Fprint(w, "GPU time: none, as the graphics driver does not time render passes\n")
	}
	fmt.Fprintln(w)

	type group struct {
		dst                   int
		width, height         int
		texWidth, texHeight   int
		screen                bool
		passes, depth, clears int
		loaded, stored        int64
		gpu                   time.Duration
	}
	var groups []group
	var passes, depth, clears int
	var loaded, stored int64
	var gpu time.Duration

	for i := range p.entries {
		e := &p.entries[i]
		if e.line != "" {
			fmt.Fprintf(w, "  %s\n", e.line)
			continue
		}
		passes++
		kind := "offscreen"
		if e.screen {
			kind = "screen"
		}
		fmt.Fprintf(w, "pass %d: dst %d (%s) %dx%d", e.index, e.dst, kind, e.width, e.height)
		if e.texWidth != e.width || e.texHeight != e.height {
			fmt.Fprintf(w, " in a %dx%d texture", e.texWidth, e.texHeight)
		}
		switch {
		case e.clearDepth:
			fmt.Fprint(w, ", depth: clears")
		case e.depth:
			fmt.Fprint(w, ", depth: loads")
		default:
			fmt.Fprint(w, ", depth: none")
		}
		var passGPU time.Duration
		if joined {
			passGPU = p.times[passes-1].GPU
			gpu += passGPU
			fmt.Fprintf(w, ", GPU: %.3f ms", ms(passGPU))
		}
		fmt.Fprintf(w, ", draws: %d, mesh draws: %d, shaders:", e.draws, e.meshes)
		for _, s := range e.shaders {
			fmt.Fprintf(w, " %d", s)
		}
		fmt.Fprintf(w, "\n  first draw: blend %s, srcs:", blendString(e.blend))
		for _, s := range e.srcs {
			if s == 0 {
				fmt.Fprint(w, " -")
				continue
			}
			fmt.Fprintf(w, " %d", s)
		}
		switch {
		case e.mesh:
			fmt.Fprint(w, ", region: whole image (mesh)\n")
		case e.region == image.Rect(0, 0, e.width, e.height):
			fmt.Fprintf(w, ", region: %v, whole image\n", e.region)
		default:
			fmt.Fprintf(w, ", region: %v, %d%% of the image\n", e.region, 100*e.region.Dx()*e.region.Dy()/max(1, e.width*e.height))
		}

		l, s := passTraffic(e)
		loaded += l
		stored += s
		if e.depth {
			depth++
		}
		if e.clearDepth {
			clears++
		}
		gi := slices.IndexFunc(groups, func(g group) bool { return g.dst == e.dst })
		if gi < 0 {
			groups = append(groups, group{dst: e.dst, width: e.width, height: e.height, texWidth: e.texWidth, texHeight: e.texHeight, screen: e.screen})
			gi = len(groups) - 1
		}
		g := &groups[gi]
		g.passes++
		if e.depth {
			g.depth++
		}
		if e.clearDepth {
			g.clears++
		}
		g.loaded += l
		g.stored += s
		g.gpu += passGPU
	}

	fmt.Fprintf(w, "\nSummary\n\n")
	fmt.Fprintf(w, "passes: %d, with depth: %d, that clear depth: %d\n", passes, depth, clears)
	fmt.Fprintf(w, "estimated bytes a frame on a tile-based GPU: loaded %.1f MB, stored %.1f MB, total %.1f MB\n", mb(loaded), mb(stored), mb(loaded+stored))
	fmt.Fprintf(w, "(4 bytes a pixel of the whole texture for colour and for depth; an offscreen pass loads and stores colour; a screen pass clears and stores colour; depth loads unless the pass clears it, and stores)\n")
	if joined {
		fmt.Fprintf(w, "GPU time of the passes: %.3f ms\n", ms(gpu))
	}
	fmt.Fprintln(w)

	slices.SortStableFunc(groups, func(a, b group) int {
		return cmp.Compare(b.loaded+b.stored, a.loaded+a.stored)
	})
	fmt.Fprintf(w, "%6s %-24s %6s %6s %6s %10s %10s %10s", "dst", "size (texture)", "passes", "depth", "clears", "loaded MB", "stored MB", "total MB")
	if joined {
		fmt.Fprintf(w, " %10s", "GPU ms")
	}
	fmt.Fprintln(w)
	for _, g := range groups {
		size := fmt.Sprintf("%dx%d", g.width, g.height)
		if g.screen {
			size += " screen"
		} else if g.texWidth != g.width || g.texHeight != g.height {
			size += fmt.Sprintf(" (%dx%d)", g.texWidth, g.texHeight)
		}
		fmt.Fprintf(w, "%6d %-24s %6d %6d %6d %10.1f %10.1f %10.1f", g.dst, size, g.passes, g.depth, g.clears, mb(g.loaded), mb(g.stored), mb(g.loaded+g.stored))
		if joined {
			fmt.Fprintf(w, " %10.3f", ms(g.gpu))
		}
		fmt.Fprintln(w)
	}

	if p.timed && !joined {
		fmt.Fprintf(w, "\nRender passes of the graphics driver, with the image IDs of the driver\n\n")
		for i, t := range p.times {
			d := "none"
			if t.Depth {
				d = "yes"
			}
			fmt.Fprintf(w, "pass %d: driver image %d %dx%d, depth: %s, GPU: %.3f ms\n", i+1, t.Dst, t.Width, t.Height, d, ms(t.GPU))
		}
	}
}

func ms(d time.Duration) float64 {
	return float64(d) / float64(time.Millisecond)
}

func mb(b int64) float64 {
	return float64(b) / 1e6
}
