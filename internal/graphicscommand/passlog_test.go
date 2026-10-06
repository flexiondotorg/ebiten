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
	"image"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/debug"
	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
)

func TestPassLogGroupsPasses(t *testing.T) {
	scene := &Image{id: 1, width: 1280, height: 800}
	road := &Image{id: 2, width: 512, height: 512}
	screen := &Image{id: 3, width: 1280, height: 800, screen: true}
	shader := &Shader{id: 7}
	full := []graphicsdriver.DstRegion{{Region: image.Rect(0, 0, 1280, 800), IndexCount: 6}}

	tri := func(dst *Image, blend graphicsdriver.Blend, src *Image) command {
		var srcs [graphics.ShaderSrcImageCount]*Image
		srcs[0] = src
		return &drawTrianglesCommand{dst: dst, srcs: srcs, blend: blend, dstRegions: full, shader: shader}
	}
	frame := []command{
		tri(scene, graphicsdriver.BlendClear, nil), // pass 1
		tri(scene, graphicsdriver.BlendSourceOver, nil),
		tri(road, graphicsdriver.BlendCopy, nil), // pass 2
		&writePixelsCommand{dst: road, args: []writePixelsCommandArgs{{region: image.Rect(0, 0, 4, 4)}}},
		tri(road, graphicsdriver.BlendSourceOver, nil), // pass 3: the write ends pass 2
		tri(screen, graphicsdriver.BlendCopy, scene),   // pass 4
	}

	path := filepath.Join(t.TempDir(), "passes.txt")
	p := newPassLogger(path, 1)
	// Frame 0 is not captured.
	p.observe(frame[:1], graphicsdriver.FlushModePresent)
	if len(p.entries) != 0 {
		t.Fatalf("frame 0: got %d entries, want none", len(p.entries))
	}
	passLog = p
	p.observe(frame[:3], graphicsdriver.FlushModeIntermediate)
	p.observe(frame[3:], graphicsdriver.FlushModePresent)
	if passLog != nil {
		passLog = nil
		t.Fatal("the pass log is still on after the captured frame")
	}

	type want struct {
		dst                 int
		draws               int
		texWidth, texHeight int
	}
	wants := []want{
		{1, 2, 2048, 1024},
		{2, 1, 512, 512},
		{2, 1, 512, 512},
		{3, 1, 1280, 800},
	}
	var got []want
	for _, e := range p.entries {
		if e.line == "" {
			got = append(got, want{e.dst, e.draws, e.texWidth, e.texHeight})
		}
	}
	if len(got) != len(wants) {
		t.Fatalf("got %d passes %+v, want %d", len(got), got, len(wants))
	}
	for i := range wants {
		if got[i] != wants[i] {
			t.Errorf("pass %d: got %+v, want %+v", i+1, got[i], wants[i])
		}
	}

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := strings.Join(strings.Fields(string(b)), " ")
	// Scene: 1 pass of 2048x1024 = 2097152 pixels, 8.4 MB each way. Road: 2 passes of 512x512,
	// 1.0 MB each way. Screen: stores 4.1 MB.
	for _, s := range []string{
		"passes: 4",
		"loaded 10.5 MB, stored 14.6 MB, total 25.1 MB",
		"1 1280x800 (2048x1024) 1 8.4 8.4 16.8",
		"3 1280x800 screen 1 0.0 4.1 4.1",
		"write-pixels: dst: 2, args: region: (0,0)-(4,4); ends the render pass",
		"flush 1 (intermediate) ends the render pass",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("the log does not contain %q:\n%s", s, b)
		}
	}
}

type passLogDriver struct {
	graphicsdriver.Graphics
}

func (passLogDriver) Begin() error                       { return nil }
func (passLogDriver) End(graphicsdriver.FlushMode) error { return nil }

func TestPassLogOffAllocatesNothing(t *testing.T) {
	var q commandQueue
	var d graphicsdriver.Graphics = passLogDriver{}
	logger := debug.SwitchFrameLogger()
	if n := testing.AllocsPerRun(100, func() {
		if err := q.flush(d, graphicsdriver.FlushModePresent, logger); err != nil {
			t.Fatal(err)
		}
	}); n != 0 {
		t.Errorf("a flush with the pass log off makes %v allocations, want 0", n)
	}
}

func TestRequestPassLog(t *testing.T) {
	t.Cleanup(func() {
		passLog = nil
		passLogRequest.Store(nil)
	})
	RequestPassLog("first.txt")
	RequestPassLog("second.txt")
	startRequestedPassLog()
	if passLog == nil || passLog.path != "second.txt" || passLog.capture != 1 {
		t.Fatalf("got the pass log %+v, want second.txt from its frame 1", passLog)
	}
	RequestPassLog("third.txt")
	startRequestedPassLog()
	if passLog.path != "second.txt" {
		t.Fatal("a request replaced the pass log that runs")
	}
	if passLogRequest.Load() == nil {
		t.Fatal("the request that waits for the running pass log is lost")
	}
}
