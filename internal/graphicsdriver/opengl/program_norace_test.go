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
)

// TestUniformCacheInvalidateAllocs checks that invalidating and setting a value again reuses the storage.
func TestUniformCacheInvalidateAllocs(t *testing.T) {
	var c uniformCache
	c.use(1)
	v := []uint32{1}
	c.set(uniformSlot(0), v)
	if n := testing.AllocsPerRun(10, func() {
		c.invalidate()
		c.set(uniformSlot(0), v)
	}); n != 0 {
		t.Errorf("allocations: got: %v, want: 0", n)
	}
}
