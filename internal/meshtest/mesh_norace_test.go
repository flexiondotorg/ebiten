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
	"image"
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

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
	// step, so that this goroutine stays blocked while they measure. The allocations are process-wide, so
	// another goroutine can add a stray one to a frame: check the average a frame, rounded as
	// testing.AllocsPerRun rounds.
	const warmup, measured = 5, 10
	var total float64
	runOnGameUpdates(warmup+measured, func(i int) {
		a := testing.AllocsPerRun(1, draw)
		if i >= warmup {
			total += a
		}
	})
	if avg := total / measured; avg >= 0.5 {
		t.Errorf("allocations: got: %v in %d frames (%v a frame), want: < 0.5 a frame", total, measured, avg)
	}
}
