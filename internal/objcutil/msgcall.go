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

// The calls without allocations also build on Linux, so that their tests can run against libc.

//go:build darwin || (linux && (amd64 || arm64))

package objcutil

import (
	"runtime"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

// maxMsgArgs is the largest number of integer words that a call passes, the receiver and the
// selector included. A struct argument passed on the stack counts one word for each 8 bytes.
const maxMsgArgs = 16

// maxStructSize is the largest struct argument in bytes.
const maxStructSize = 64

// msgCall is the argument array of a call to a C function through purego.SyscallN.
//
// purego.SyscallN is go:uintptrescapes, so a variadic argument array on the stack moves to the
// heap at each call, and objc.ID.Send allocates through reflection. A call takes a msgCall from
// msgCallPool instead, so that the argument array is on the heap already. The pool is safe for
// the main thread, the rendering thread, and the threads of Core Animation.
type msgCall struct {
	args [maxMsgArgs]uintptr

	// structArg holds the copy of a struct argument that the caller passes by reference.
	structArg [maxStructSize / 8]uintptr

	pinner runtime.Pinner
}

var msgCallPool = sync.Pool{
	New: func() any {
		return &msgCall{}
	},
}

// setArgs copies the integer arguments to c.args and returns their number.
func (c *msgCall) setArgs(a0, a1 uintptr, args []uintptr) int {
	if 2+len(args) > len(c.args) {
		panic("objcutil: too many arguments")
	}
	c.args[0] = a0
	c.args[1] = a1
	return 2 + copy(c.args[2:], args)
}

func (c *msgCall) call(fn uintptr, n int) uintptr {
	r, _, _ := purego.SyscallN(fn, c.args[:n]...)
	return r
}

// call calls fn with integer and pointer-sized arguments that hold no Go pointer, and returns the
// value of the integer result register.
//
// Floating-point arguments and results do not work: purego.SyscallN copies the integer arguments
// to the floating-point registers, and it does not return the floating-point result.
func call(fn uintptr, a0, a1 uintptr, args ...uintptr) uintptr {
	c := msgCallPool.Get().(*msgCall)
	r := c.call(fn, c.setArgs(a0, a1, args))
	msgCallPool.Put(c)
	return r
}

// callPointer is like call, with the Go pointer p as the third argument, after a0 and a1.
// p is pinned during the call, so the function must not keep it after it returns.
func callPointer(fn uintptr, a0, a1 uintptr, p unsafe.Pointer, args ...uintptr) uintptr {
	c := msgCallPool.Get().(*msgCall)
	if 3+len(args) > len(c.args) {
		panic("objcutil: too many arguments")
	}
	c.args[0] = a0
	c.args[1] = a1
	c.args[2] = uintptr(p)
	n := 3 + copy(c.args[3:], args)
	c.pinner.Pin(p)
	r := c.call(fn, n)
	c.pinner.Unpin()
	msgCallPool.Put(c)
	return r
}

// callOut is like call, with the address of a result area in the call record as the third argument,
// after a0 and a1. The function writes at most size bytes there, and callOut copies them to out, so
// that out can be on the stack of the caller.
func callOut(fn uintptr, a0, a1 uintptr, out unsafe.Pointer, size uintptr, args ...uintptr) uintptr {
	if size > maxStructSize {
		panic("objcutil: unsupported result size")
	}
	c := msgCallPool.Get().(*msgCall)
	if 3+len(args) > len(c.args) {
		panic("objcutil: too many arguments")
	}
	clear(c.structArg[:])
	c.args[0] = a0
	c.args[1] = a1
	c.args[2] = uintptr(unsafe.Pointer(&c.structArg))
	n := 3 + copy(c.args[3:], args)
	c.pinner.Pin(c)
	r := c.call(fn, n)
	c.pinner.Unpin()
	copy(unsafe.Slice((*byte)(out), size), unsafe.Slice((*byte)(unsafe.Pointer(&c.structArg)), size))
	msgCallPool.Put(c)
	return r
}

// callStruct is like call, with a struct argument by value after the integer arguments a0, a1,
// and args. The struct must be the last argument. It is size bytes at s, and it must be larger
// than 16 bytes and a multiple of 8 bytes, so that the platform passes it in memory: by reference
// on arm64 and on the stack on amd64 (see packStruct). On arm64, a struct of at most 4 members of
// one floating-point type travels in the floating-point registers instead, so it must not use
// callStruct.
func callStruct(fn uintptr, a0, a1 uintptr, s unsafe.Pointer, size uintptr, args ...uintptr) uintptr {
	if size <= 16 || size > maxStructSize || size%8 != 0 {
		panic("objcutil: unsupported struct size")
	}
	c := msgCallPool.Get().(*msgCall)
	n := c.setArgs(a0, a1, args)
	words := unsafe.Slice((*uintptr)(s), size/8)
	n = c.packStruct(n, words)
	r := c.call(fn, n)
	c.pinner.Unpin()
	msgCallPool.Put(c)
	return r
}
