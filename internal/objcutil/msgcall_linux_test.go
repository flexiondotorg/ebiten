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

//go:build amd64 || arm64

package objcutil

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
)

func libc(t *testing.T, name string) uintptr {
	t.Helper()
	lib, err := purego.Dlopen("libc.so.6", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		t.Skipf("dlopen libc: %v", err)
	}
	fn, err := purego.Dlsym(lib, name)
	if err != nil {
		t.Fatal(err)
	}
	return fn
}

// cString returns a NUL-terminated copy of s on the heap.
func cString(s string) []byte {
	return append([]byte(s), 0)
}

// goString returns the bytes of buf before the first NUL.
func goString(buf []byte) string {
	for i, b := range buf {
		if b == 0 {
			return string(buf[:i])
		}
	}
	return string(buf)
}

func TestCall(t *testing.T) {
	labs := libc(t, "labs")
	v := -12345
	if got := int(call(labs, uintptr(v), 0)); got != 12345 {
		t.Errorf("labs: got %d, want 12345", got)
	}
	if n := testing.AllocsPerRun(100, func() {
		call(labs, uintptr(v), 0)
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}

func TestCallPointer(t *testing.T) {
	snprintf := libc(t, "snprintf")
	buf := make([]byte, 64)
	format := cString("value %ld")
	var p runtime.Pinner
	p.Pin(&buf[0])
	defer p.Unpin()

	callPointer(snprintf, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), unsafe.Pointer(&format[0]), 42)
	if got, want := goString(buf), "value 42"; got != want {
		t.Errorf("snprintf: got %q, want %q", got, want)
	}
	if n := testing.AllocsPerRun(100, func() {
		callPointer(snprintf, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), unsafe.Pointer(&format[0]), 42)
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}

func TestCallTooManyArguments(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("no panic")
		}
	}()
	var args [maxMsgArgs - 1]uintptr
	call(1, 0, 0, args[:]...)
}

func TestCallStructSize(t *testing.T) {
	for _, size := range []uintptr{8, 16, 20, maxStructSize + 8} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("size %d: no panic", size)
				}
			}()
			var s [maxStructSize/8 + 1]uintptr
			callStruct(1, 0, 0, unsafe.Pointer(&s[0]), size)
		}()
	}
}
