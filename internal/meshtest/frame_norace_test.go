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

//go:build !race

// The allocation tests are excluded from race builds: with -race, sync.Pool.Put deliberately drops entries, so these
// calls can allocate.

package meshtest_test

import (
	"fmt"
	"image"
	"os"
	"runtime"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

// frameAllocsCeiling is the most heap allocations a frame that any platform may have.
const frameAllocsCeiling = 2000

// frameIdleBaselines and frameMeshBaselines are the heap allocations a frame that each GOOS measured, for an idle
// frame and for a frame with 8 mesh draws. A GOOS without a baseline has only the ceiling. The values are those of
// the branch without the zero-allocation patches. The commit that tests those patches lowers the macOS values.
var (
	frameIdleBaselines = map[string]float64{
		"linux":  48,
		"darwin": 818,
	}
	frameMeshBaselines = map[string]float64{
		"linux":  48,
		"darwin": 1980,
	}
)

// checkFrameAllocs measures the heap allocations a frame, process-wide, over whole frames: Update, Draw, the flush,
// and the present. draw runs in the Update of each frame.
//
// The test fails above frameAllocsCeiling. Above the baseline of the GOOS, it logs a warning, and it fails if the
// environment variable EBITENGINE_ALLOC_STRICT is 1.
func checkFrameAllocs(t *testing.T, baselines map[string]float64, draw func()) {
	// The first frames grow the pools and the buffers of each command queue.
	const warmup, measured = 10, 120
	var ms runtime.MemStats
	var start, end uint64
	runOnGameUpdates(warmup+measured+1, func(i int) {
		switch i {
		case warmup:
			runtime.ReadMemStats(&ms)
			start = ms.Mallocs
		case warmup + measured:
			runtime.ReadMemStats(&ms)
			end = ms.Mallocs
			return
		}
		if draw != nil {
			draw()
		}
	})

	avg := float64(end-start) / measured
	t.Logf("allocations: %v a frame (%d in %d frames)", avg, end-start, measured)
	if avg > frameAllocsCeiling {
		t.Errorf("allocations: got: %v a frame, want: <= %d a frame", avg, frameAllocsCeiling)
	}

	baseline, ok := baselines[runtime.GOOS]
	if !ok || avg <= baseline {
		return
	}
	msg := fmt.Sprintf("WARNING: allocations: got: %v a frame, baseline: %v a frame (%+.1f%%)", avg, baseline, (avg-baseline)/baseline*100)
	if os.Getenv("EBITENGINE_ALLOC_STRICT") == "1" {
		t.Errorf("%s", msg)
		return
	}
	t.Logf("%s", msg)
}

// TestFrameAllocationsIdle checks the heap allocations of a frame that draws nothing.
func TestFrameAllocationsIdle(t *testing.T) {
	checkFrameAllocs(t, frameIdleBaselines, nil)
}

// TestFrameAllocationsMesh checks the heap allocations of a frame with 8 mesh draws with instances and Depth.
func TestFrameAllocationsMesh(t *testing.T) {
	var s *ebiten.Shader
	var dst *ebiten.Image
	instances := []ebiten.Vertex{
		instance(8, 8, farZ, red),
		instance(16, 16, nearZ, blue),
		instance(24, 24, farZ, green),
	}
	op := &ebiten.DrawTrianglesShaderOptions{
		Depth: true,
	}
	runOnGameUpdate(func() {
		var err error
		s, err = ebiten.NewShader([]byte(meshShaderSource))
		if err != nil {
			t.Error(err)
			return
		}
		op.Mesh = newQuadMesh()
		dst = ebiten.NewImageWithOptions(image.Rect(0, 0, 64, 64), &ebiten.NewImageOptions{Unmanaged: true})
	})
	if t.Failed() {
		return
	}

	checkFrameAllocs(t, frameMeshBaselines, func() {
		for range 8 {
			dst.DrawTrianglesShader32(instances, nil, s, op)
		}
	})
}
