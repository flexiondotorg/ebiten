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

package objcutil

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestCallStructAllocs(t *testing.T) {
	snprintf := libc(t, "snprintf")
	buf := make([]byte, 64)
	format := cString("%lx %lx %lx %lx %lx %lx %lx")
	var p runtime.Pinner
	p.Pin(&buf[0])
	p.Pin(&format[0])
	defer p.Unpin()

	r := rect{x: 0x11, y: 0x22, width: 0x33, height: 0x44}
	if n := testing.AllocsPerRun(100, func() {
		callStruct(snprintf, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), unsafe.Pointer(&r), unsafe.Sizeof(r), uintptr(unsafe.Pointer(&format[0])))
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}
