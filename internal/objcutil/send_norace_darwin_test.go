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

package objcutil_test

import (
	"runtime"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/hajimehoshi/ebiten/v2/internal/cocoa"
	"github.com/hajimehoshi/ebiten/v2/internal/objcutil"
)

func TestSendAllocs(t *testing.T) {
	obj, other := newObject(t, "NSObject"), newObject(t, "NSObject")
	selHash := objc.RegisterName("hash")
	selIsEqual := objc.RegisterName("isEqual:")

	if n := testing.AllocsPerRun(100, func() {
		objcutil.Send(obj, selHash)
		objcutil.Send(obj, selIsEqual, uintptr(other))
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}

func TestSendPointerAllocs(t *testing.T) {
	data := newObject(t, "NSMutableData")
	selAppendBytesLength := objc.RegisterName("appendBytes:length:")

	src := []byte("abcdef")
	if n := testing.AllocsPerRun(100, func() {
		objcutil.SendPointer(data, selAppendBytesLength, unsafe.Pointer(&src[0]), uintptr(len(src)))
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}

func TestSendStructAllocs(t *testing.T) {
	transform := newObject(t, "NSAffineTransform")
	selSetTransformStruct := objc.RegisterName("setTransformStruct:")

	want := affineTransformStruct{m11: 1.5, m12: 2.5, m21: 3.5, m22: 4.5, tX: 5.5, tY: 6.5}
	if n := testing.AllocsPerRun(100, func() {
		objcutil.SendStruct(transform, selSetTransformStruct, unsafe.Pointer(&want), unsafe.Sizeof(want))
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}

func TestValueForKeyAllocs(t *testing.T) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	loadFoundation(t)
	selSetTransformStruct := objc.RegisterName("setTransformStruct:")

	pool := cocoa.NSAutoreleasePool_new()
	defer pool.Release()

	transform := newObject(t, "NSAffineTransform")
	want := affineTransformStruct{m11: 1.5, m12: 2.5, m21: 3.5, m22: 4.5, tX: 5.5, tY: 6.5}
	objcutil.SendStruct(transform, selSetTransformStruct, unsafe.Pointer(&want), unsafe.Sizeof(want))
	keyTransformStruct := newKey(t, "transformStruct")
	number := newNumber(2.75)
	keyDoubleValue := newKey(t, "doubleValue")

	if n := testing.AllocsPerRun(100, func() {
		var got affineTransformStruct
		objcutil.ValueForKey(transform, keyTransformStruct, unsafe.Pointer(&got), unsafe.Sizeof(got))
		var d float64
		objcutil.ValueForKey(number, keyDoubleValue, unsafe.Pointer(&d), unsafe.Sizeof(d))
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}
