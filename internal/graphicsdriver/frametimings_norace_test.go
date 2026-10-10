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

package graphicsdriver

import (
	"testing"
)

func TestFrameTimingsAllocations(t *testing.T) {
	var f FrameTimings
	var dst [4]FrameTiming
	f.Read(nil)
	var frame int64
	allocs := testing.AllocsPerRun(100, func() {
		runFrame(&f, frame, true)
		frame++
		f.Read(dst[:])
	})
	if allocs != 0 {
		t.Errorf("got %v allocations a frame, want 0", allocs)
	}
}
