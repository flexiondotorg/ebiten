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

//go:build !playstation5 && !race

// The allocation tests are excluded from race builds: with -race, sync.Pool.Put deliberately drops entries, so these
// calls can allocate.

package opengl

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
)

func TestFrameTimingsAllocations(t *testing.T) {
	g, c := newTimedGraphics()
	a := &Image{id: 1, width: 64, height: 32}
	b := &Image{id: 2, width: 128, height: 64}
	var dst [8]graphicsdriver.FrameTiming
	g.ReadFrameTimings(dst[:])
	runTimedFrame(g, a, b)
	allocs := testing.AllocsPerRun(100, func() {
		runTimedFrame(g, a, b)
		c.finishGPU()
		g.ReadFrameTimings(dst[:])
	})
	if allocs != 0 {
		t.Errorf("got %v allocations a frame, want 0", allocs)
	}
}
