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

package objcutil

import (
	"runtime"
	"slices"
	"testing"
	"unsafe"
)

type rect struct {
	x, y, width, height uintptr
}

// TestCallStruct checks that the words of a struct argument are the first words on the stack, as
// the System V ABI passes a struct of more than 16 bytes. snprintf reads its arguments after the
// format from the 3 remaining registers and then from the stack, so it prints the zeros of the
// registers and then the struct.
func TestCallStruct(t *testing.T) {
	snprintf := libc(t, "snprintf")
	buf := make([]byte, 64)
	format := cString("%lx %lx %lx %lx %lx %lx %lx")
	var p runtime.Pinner
	p.Pin(&buf[0])
	p.Pin(&format[0])
	defer p.Unpin()

	r := rect{x: 0x11, y: 0x22, width: 0x33, height: 0x44}
	callStruct(snprintf, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), unsafe.Pointer(&r), unsafe.Sizeof(r), uintptr(unsafe.Pointer(&format[0])))
	if got, want := goString(buf), "0 0 0 11 22 33 44"; got != want {
		t.Errorf("snprintf: got %q, want %q", got, want)
	}
	if n := testing.AllocsPerRun(100, func() {
		callStruct(snprintf, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)), unsafe.Pointer(&r), unsafe.Sizeof(r), uintptr(unsafe.Pointer(&format[0])))
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}

func TestPackStruct(t *testing.T) {
	words := []uintptr{0xa, 0xb, 0xc, 0xd}
	for _, tc := range []struct {
		n    int
		want []uintptr
	}{
		{n: 2, want: []uintptr{1, 2, 0, 0, 0, 0, 0xa, 0xb, 0xc, 0xd}},
		{n: 6, want: []uintptr{1, 2, 3, 4, 5, 6, 0xa, 0xb, 0xc, 0xd}},
		{n: 7, want: []uintptr{1, 2, 3, 4, 5, 6, 7, 0xa, 0xb, 0xc, 0xd}},
	} {
		var c msgCall
		for i := range c.args {
			c.args[i] = 0xff
		}
		for i := range tc.n {
			c.args[i] = uintptr(i + 1)
		}
		n := c.packStruct(tc.n, words)
		if got := c.args[:n]; !slices.Equal(got, tc.want) {
			t.Errorf("n %d: got %x, want %x", tc.n, got, tc.want)
		}
	}
}
