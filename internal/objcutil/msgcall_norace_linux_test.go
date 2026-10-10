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

//go:build (amd64 || arm64) && !race

// The allocation tests are excluded from race builds: with -race, sync.Pool.Put deliberately drops entries, so these
// calls can allocate.

package objcutil

import (
	"runtime"
	"testing"
	"unsafe"
)

func TestCallAllocs(t *testing.T) {
	labs := libc(t, "labs")
	v := -12345
	if n := testing.AllocsPerRun(100, func() {
		call(labs, uintptr(v), 0)
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}

func TestCallPointerAllocs(t *testing.T) {
	snprintf := libc(t, "snprintf")
	buf := make([]byte, 64)
	format := cString("value %ld")
	var p runtime.Pinner
	p.Pin(&buf[0])
	defer p.Unpin()

	if n := testing.AllocsPerRun(100, func() {
		callPointer(snprintf, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), unsafe.Pointer(&format[0]), 42)
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}

func TestCallOutAllocs(t *testing.T) {
	inetNtop := libc(t, "inet_ntop")
	const afInet = 2
	src := []byte{192, 168, 10, 200}
	var p runtime.Pinner
	p.Pin(&src[0])
	defer p.Unpin()

	if n := testing.AllocsPerRun(100, func() {
		var out [16]byte
		callOut(inetNtop, afInet, uintptr(unsafe.Pointer(&src[0])), unsafe.Pointer(&out), uintptr(len(out)), uintptr(len(out)))
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}
