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
	"time"

	"github.com/hajimehoshi/ebiten/v2/internal/debug"
	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
)

func TestPassLogGroupsPasses(t *testing.T) {
	scene := &Image{id: 1, width: 1280, height: 800}
	road := &Image{id: 2, width: 512, height: 512}
	screen := &Image{id: 3, width: 1280, height: 800, screen: true}
	shader := &Shader{id: 7}
	mesh := &Mesh{id: 1}
	full := []graphicsdriver.DstRegion{{Region: image.Rect(0, 0, 1280, 800), IndexCount: 6}}

	tri := func(dst *Image, blend graphicsdriver.Blend, src *Image) command {
		var srcs [graphics.ShaderSrcImageCount]*Image
		srcs[0] = src
		return &drawTrianglesCommand{dst: dst, srcs: srcs, blend: blend, dstRegions: full, shader: shader}
	}
	meshDraw := func(dst *Image) command {
		return &drawMeshCommand{dst: dst, mesh: mesh, instances: make([]float32, graphics.VertexFloatCount), shader: shader}
	}
	frame := []command{
		tri(scene, graphicsdriver.BlendClear, nil), // pass 1
		tri(scene, graphicsdriver.BlendSourceOver, nil),
		meshDraw(scene), // same pass
		meshDraw(scene),
		tri(road, graphicsdriver.BlendCopy, nil), // pass 2
		&writePixelsCommand{dst: road, args: []writePixelsCommandArgs{{region: image.Rect(0, 0, 4, 4)}}},
		tri(road, graphicsdriver.BlendSourceOver, nil), // pass 3: the write ends pass 2
		tri(screen, graphicsdriver.BlendCopy, scene),   // pass 4
	}

	path := filepath.Join(t.TempDir(), "passes.txt")
	p := newPassLogger(path, 1)
	// Frame 0 is not captured.
	p.observe(frame[:1], graphicsdriver.FlushModePresent, nil)
	if len(p.entries) != 0 {
		t.Fatalf("frame 0: got %d entries, want none", len(p.entries))
	}
	passLog = p
	p.observe(frame[:5], graphicsdriver.FlushModeIntermediate, nil)
	p.observe(frame[5:], graphicsdriver.FlushModePresent, nil)
	if passLog != nil {
		passLog = nil
		t.Fatal("the pass log is still on after the captured frame")
	}

	type want struct {
		dst                 int
		draws, meshes       int
		texWidth, texHeight int
	}
	wants := []want{
		{1, 4, 2, 2048, 1024},
		{2, 1, 0, 512, 512},
		{2, 1, 0, 512, 512},
		{3, 1, 0, 1280, 800},
	}
	var got []want
	for _, e := range p.entries {
		if e.line == "" {
			got = append(got, want{e.dst, e.draws, e.meshes, e.texWidth, e.texHeight})
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
		"draws: 4, mesh draws: 2, shaders: 7",
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

type passTimerDriver struct {
	graphicsdriver.Graphics
	started bool
	ready   bool
	times   []graphicsdriver.PassTime
}

func (d *passTimerDriver) TimePasses() bool {
	d.started = true
	return true
}

func (d *passTimerDriver) ReadPassTimes(dst []graphicsdriver.PassTime) ([]graphicsdriver.PassTime, bool) {
	if !d.ready {
		return dst, false
	}
	return append(dst, d.times...), true
}

// capturePassTimes captures one frame of two passes with d and returns the log, which waits for one
// frame for the pass times.
func capturePassTimes(t *testing.T, d *passTimerDriver) string {
	t.Helper()
	scene := &Image{id: 1, width: 1280, height: 800}
	screen := &Image{id: 2, width: 1280, height: 800, screen: true}
	full := []graphicsdriver.DstRegion{{Region: image.Rect(0, 0, 1280, 800), IndexCount: 6}}
	shader := &Shader{id: 7}
	frame := []command{
		&drawTrianglesCommand{dst: scene, blend: graphicsdriver.BlendCopy, dstRegions: full, shader: shader},
		&drawTrianglesCommand{dst: screen, blend: graphicsdriver.BlendCopy, dstRegions: full, shader: shader},
	}

	path := filepath.Join(t.TempDir(), "passes.txt")
	p := newPassLogger(path, 0)
	passLog = p
	t.Cleanup(func() { passLog = nil })
	p.observe(frame, graphicsdriver.FlushModePresent, d)
	if !d.started {
		t.Fatal("the pass log did not ask the driver to time the passes")
	}
	p.observe(nil, graphicsdriver.FlushModePresent, d)
	if passLog == nil {
		t.Fatal("the pass log is written before the pass times are ready")
	}
	d.ready = true
	p.observe(nil, graphicsdriver.FlushModePresent, d)
	if passLog != nil {
		t.Fatal("the pass log is still on after the pass times are ready")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestPassLogTimesPasses(t *testing.T) {
	d := &passTimerDriver{times: []graphicsdriver.PassTime{
		{Dst: 11, Width: 1280, Height: 800, GPU: 1500 * time.Microsecond},
		{Dst: 12, Width: 1280, Height: 800, GPU: 250 * time.Microsecond},
	}}
	b := capturePassTimes(t, d)
	out := strings.Join(strings.Fields(b), " ")
	for _, s := range []string{
		"GPU time: the GPU time of each pass",
		"pass 1: dst 1 (offscreen) 1280x800 in a 2048x1024 texture, GPU: 1.500 ms",
		"pass 2: dst 2 (screen) 1280x800, GPU: 0.250 ms",
		"GPU time of the passes: 1.750 ms",
		"total MB GPU ms",
		"1 1280x800 (2048x1024) 1 8.4 8.4 16.8 1.500",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("the log does not contain %q:\n%s", s, b)
		}
	}
	if strings.Contains(out, "driver image") {
		t.Errorf("the log lists the driver passes, which match:\n%s", b)
	}
}

func TestPassLogListsUnmatchedPassTimes(t *testing.T) {
	d := &passTimerDriver{times: []graphicsdriver.PassTime{
		{Dst: 11, Width: 640, Height: 400, GPU: 1500 * time.Microsecond},
		{Dst: 12, Width: 1280, Height: 800, GPU: 250 * time.Microsecond},
	}}
	b := capturePassTimes(t, d)
	out := strings.Join(strings.Fields(b), " ")
	for _, s := range []string{
		"the graphics driver made 2 passes, which do not match",
		"pass 1: driver image 11 640x400, GPU: 1.500 ms",
		"pass 2: driver image 12 1280x800, GPU: 0.250 ms",
	} {
		if !strings.Contains(out, s) {
			t.Errorf("the log does not contain %q:\n%s", s, b)
		}
	}
	if strings.Contains(out, "GPU ms") {
		t.Errorf("the log joins passes that do not match:\n%s", b)
	}
}

func TestPassLogWithoutPassTimes(t *testing.T) {
	d := &passTimerDriver{}
	scene := &Image{id: 1, width: 64, height: 64}
	full := []graphicsdriver.DstRegion{{Region: image.Rect(0, 0, 64, 64), IndexCount: 6}}
	path := filepath.Join(t.TempDir(), "passes.txt")
	p := newPassLogger(path, 0)
	passLog = p
	t.Cleanup(func() { passLog = nil })
	p.observe([]command{&drawTrianglesCommand{dst: scene, blend: graphicsdriver.BlendCopy, dstRegions: full, shader: &Shader{id: 1}}}, graphicsdriver.FlushModePresent, d)
	for range passLogWait {
		if passLog == nil {
			t.Fatal("the pass log is written before the wait ends")
		}
		p.observe(nil, graphicsdriver.FlushModePresent, d)
	}
	if passLog != nil {
		t.Fatal("the pass log is still on after the wait")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "GPU time: none, as the GPU did not complete the frame") {
		t.Errorf("the log does not say that the pass times are missing:\n%s", b)
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
