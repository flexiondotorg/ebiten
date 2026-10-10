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

package graphicscommand

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/debug"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
)

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
