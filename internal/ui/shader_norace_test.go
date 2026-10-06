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

package ui

import (
	"testing"
)

// TestAppendUniformsFastTypesAllocs checks that the types without reflection do not allocate.
func TestAppendUniformsFastTypesAllocs(t *testing.T) {
	s := newTestShader()
	fast := map[string]any{
		"F": float32(1.5),
		"I": -2,
		"V": []float32{1, -2, 3.25, 4},
		"A": []int{5, -6, 7},
	}
	dst := make([]uint32, 0, 64)
	if n := testing.AllocsPerRun(100, func() { dst = s.AppendUniforms(dst[:0], fast) }); n != 0 {
		t.Errorf("AppendUniforms allocated %v times, want 0", n)
	}
}
