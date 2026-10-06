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

	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
)

// The render-pass log is a diagnostic hook. RequestPassLog asks for the log of one frame, and
// ebiten.CaptureRenderPasses calls it.
//
// The log groups the draw commands of one frame into render passes as the Metal driver does: a new
// pass at each flush, after a pixel write or read, and when the destination changes. The command
// stream is the same on every graphics driver, so a run with any driver gives the passes that Metal
// makes.

// passLog is nil when the log is off, and after the log is written. Only the render thread uses it.
var passLog *passLogger

// passLogRequest holds the path of the pass log that RequestPassLog asks for, until the end of a
// frame starts it.
var passLogRequest atomic.Pointer[string]

// RequestPassLog asks for a pass log of one frame, written to path. The log starts at the end of
// the next frame while no other pass log runs, and captures its second frame. A later request
// replaces a request that waits.
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

	lastDst *Image
	entries []passEntry
	passes  int
	flushes int
}

// passEntry is a render pass, or a line for a command or a flush outside the render passes.
type passEntry struct {
	// line is not empty for an entry that is not a render pass.
	line string

	index     int
	dst       int
	width     int
	height    int
	texWidth  int
	texHeight int
	screen    bool
	draws     int
	shaders   []int

	// The first draw of the pass.
	blend  graphicsdriver.Blend
	srcs   [graphics.ShaderSrcImageCount]int
	region image.Rectangle
}

// observe records the commands of one flush. The commands are not executed yet.
func (p *passLogger) observe(commands []command, mode graphicsdriver.FlushMode) {
	if !p.inFrame {
		p.inFrame = true
		p.capturing = p.frame == p.capture
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
		p.observeDraw(c.dst, c.shader.id, c.blend, srcs, c.dstRegions[0].Region)
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

func (p *passLogger) observeDraw(dst *Image, shader int, blend graphicsdriver.Blend, srcs [graphics.ShaderSrcImageCount]int, region image.Rectangle) {
	if !p.capturing {
		return
	}

	if p.lastDst != dst {
		p.passes++
		e := passEntry{
			index:     p.passes,
			dst:       dst.id,
			width:     dst.width,
			height:    dst.height,
			texWidth:  dst.width,
			texHeight: dst.height,
			screen:    dst.screen,
			blend:     blend,
			srcs:      srcs,
			region:    region,
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
	if !slices.Contains(e.shaders, shader) {
		e.shaders = append(e.shaders, shader)
	}
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
// a pixel of the whole texture. An offscreen pass loads and stores the colour image, and a screen pass
// clears it and stores it.
func passTraffic(e *passEntry) (loaded, stored int64) {
	px := int64(e.texWidth) * int64(e.texHeight)
	stored = 4 * px
	if !e.screen {
		loaded = 4 * px
	}
	return loaded, stored
}

func (p *passLogger) write(w *bytes.Buffer) {
	fmt.Fprintf(w, "Render passes of frame %d, grouped as the Metal driver groups them\n\n", p.frame)

	type group struct {
		dst                 int
		width, height       int
		texWidth, texHeight int
		screen              bool
		passes              int
		loaded, stored      int64
	}
	var groups []group
	var passes int
	var loaded, stored int64

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
		fmt.Fprintf(w, ", draws: %d, shaders:", e.draws)
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
		if e.region == image.Rect(0, 0, e.width, e.height) {
			fmt.Fprintf(w, ", region: %v, whole image\n", e.region)
		} else {
			fmt.Fprintf(w, ", region: %v, %d%% of the image\n", e.region, 100*e.region.Dx()*e.region.Dy()/max(1, e.width*e.height))
		}

		l, s := passTraffic(e)
		loaded += l
		stored += s
		gi := slices.IndexFunc(groups, func(g group) bool { return g.dst == e.dst })
		if gi < 0 {
			groups = append(groups, group{dst: e.dst, width: e.width, height: e.height, texWidth: e.texWidth, texHeight: e.texHeight, screen: e.screen})
			gi = len(groups) - 1
		}
		g := &groups[gi]
		g.passes++
		g.loaded += l
		g.stored += s
	}

	fmt.Fprintf(w, "\nSummary\n\n")
	fmt.Fprintf(w, "passes: %d\n", passes)
	fmt.Fprintf(w, "estimated bytes a frame on a tile-based GPU: loaded %.1f MB, stored %.1f MB, total %.1f MB\n", mb(loaded), mb(stored), mb(loaded+stored))
	fmt.Fprintf(w, "(4 bytes a pixel of the whole texture; an offscreen pass loads and stores colour; a screen pass clears and stores colour)\n\n")

	slices.SortStableFunc(groups, func(a, b group) int {
		return cmp.Compare(b.loaded+b.stored, a.loaded+a.stored)
	})
	fmt.Fprintf(w, "%6s %-24s %6s %10s %10s %10s\n", "dst", "size (texture)", "passes", "loaded MB", "stored MB", "total MB")
	for _, g := range groups {
		size := fmt.Sprintf("%dx%d", g.width, g.height)
		if g.screen {
			size += " screen"
		} else if g.texWidth != g.width || g.texHeight != g.height {
			size += fmt.Sprintf(" (%dx%d)", g.texWidth, g.texHeight)
		}
		fmt.Fprintf(w, "%6d %-24s %6d %10.1f %10.1f %10.1f\n", g.dst, size, g.passes, mb(g.loaded), mb(g.stored), mb(g.loaded+g.stored))
	}
}

func mb(b int64) float64 {
	return float64(b) / 1e6
}
