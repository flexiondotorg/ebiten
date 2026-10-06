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
	"fmt"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"
)

// msgSend is the address of objc_msgSend.
var msgSend uintptr

func init() {
	lib, err := purego.Dlopen("/usr/lib/libobjc.A.dylib", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if err != nil {
		panic(fmt.Errorf("objcutil: %w", err))
	}
	msgSend, err = purego.Dlsym(lib, "objc_msgSend")
	if err != nil {
		panic(fmt.Errorf("objcutil: %w", err))
	}
}

// Send sends a message to id and returns the value of the integer result register, without
// allocations. It is for the messages of each frame, in place of objc.ID.Send, which allocates
// through reflection.
//
// Every argument must be an integer, an object, a selector, or a pointer that is not a Go
// pointer, and the result must be void, an integer, an object, or a pointer. Floating-point
// arguments and results do not work. Mask a BOOL result with 0xff.
func Send(id objc.ID, sel objc.SEL, args ...uintptr) uintptr {
	return call(msgSend, uintptr(id), uintptr(sel), args...)
}

// SendPointer is like Send, with the Go pointer p as the first argument after the selector.
// p is pinned during the call, so the method must not keep it after it returns, for example a
// method that copies the bytes at p.
func SendPointer(id objc.ID, sel objc.SEL, p unsafe.Pointer, args ...uintptr) uintptr {
	return callPointer(msgSend, uintptr(id), uintptr(sel), p, args...)
}

// SendStruct is like Send, with one argument: a struct by value, size bytes at s. The struct must
// be larger than 16 bytes and a multiple of 8 bytes, for example MTLViewport or MTLScissorRect.
// Such a struct travels in memory on arm64 and amd64, so floating-point fields work, apart from
// one exception: on arm64, a struct of at most 4 members of one floating-point type, for example
// MTLClearColor or NSRect, travels in the floating-point registers, and SendStruct must not pass it.
func SendStruct(id objc.ID, sel objc.SEL, s unsafe.Pointer, size uintptr) uintptr {
	return callStruct(msgSend, uintptr(id), uintptr(sel), s, size)
}
