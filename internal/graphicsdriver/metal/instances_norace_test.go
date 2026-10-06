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

package metal

import (
	"testing"
)

func TestInstanceSpaceAllocations(t *testing.T) {
	var s instanceSpace
	frame := int64(0)
	if n := testing.AllocsPerRun(100, func() {
		for range 300 {
			if _, newLength := s.place(frame, 48); newLength != 0 {
				s.use(pow2(newLength), 48)
			}
		}
		frame++
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}
