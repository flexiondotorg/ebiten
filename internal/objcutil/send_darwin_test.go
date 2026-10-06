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

package objcutil_test

import (
	"structs"
	"testing"
	"unsafe"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/hajimehoshi/ebiten/v2/internal/objcutil"
)

func loadFoundation(t *testing.T) {
	t.Helper()
	if _, err := purego.Dlopen("/System/Library/Frameworks/Foundation.framework/Foundation", purego.RTLD_NOW|purego.RTLD_GLOBAL); err != nil {
		t.Fatal(err)
	}
}

func TestSend(t *testing.T) {
	loadFoundation(t)
	selNew := objc.RegisterName("new")
	selRelease := objc.RegisterName("release")
	selHash := objc.RegisterName("hash")
	selRetainCount := objc.RegisterName("retainCount")
	selIsEqual := objc.RegisterName("isEqual:")

	obj := objc.ID(objc.GetClass("NSObject")).Send(selNew)
	other := objc.ID(objc.GetClass("NSObject")).Send(selNew)
	defer obj.Send(selRelease)
	defer other.Send(selRelease)

	if got, want := objcutil.Send(obj, selHash), uintptr(obj.Send(selHash)); got != want {
		t.Errorf("hash: got %d, want %d", got, want)
	}
	if got, want := objcutil.Send(obj, selRetainCount), uintptr(obj.Send(selRetainCount)); got != want {
		t.Errorf("retainCount: got %d, want %d", got, want)
	}
	if got := objcutil.Send(obj, selIsEqual, uintptr(obj)) & 0xff; got != 1 {
		t.Errorf("isEqual: itself: got %d, want 1", got)
	}
	if got := objcutil.Send(obj, selIsEqual, uintptr(other)) & 0xff; got != 0 {
		t.Errorf("isEqual: another object: got %d, want 0", got)
	}

	if n := testing.AllocsPerRun(100, func() {
		objcutil.Send(obj, selHash)
		objcutil.Send(obj, selIsEqual, uintptr(other))
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}

func TestSendPointer(t *testing.T) {
	loadFoundation(t)
	selNew := objc.RegisterName("new")
	selRelease := objc.RegisterName("release")
	selAppendBytesLength := objc.RegisterName("appendBytes:length:")
	selLength := objc.RegisterName("length")
	selBytes := objc.RegisterName("bytes")

	data := objc.ID(objc.GetClass("NSMutableData")).Send(selNew)
	defer data.Send(selRelease)

	src := []byte("abcdef")
	objcutil.SendPointer(data, selAppendBytesLength, unsafe.Pointer(&src[0]), uintptr(len(src)))
	if got := objcutil.Send(data, selLength); got != uintptr(len(src)) {
		t.Fatalf("length: got %d, want %d", got, len(src))
	}
	p := objcutil.Send(data, selBytes)
	if got := string(unsafe.Slice(*(**byte)(unsafe.Pointer(&p)), len(src))); got != string(src) {
		t.Errorf("bytes: got %q, want %q", got, src)
	}

	if n := testing.AllocsPerRun(100, func() {
		objcutil.SendPointer(data, selAppendBytesLength, unsafe.Pointer(&src[0]), uintptr(len(src)))
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}

// affineTransformStruct is NSAffineTransformStruct: 6 doubles, the same form as MTLViewport.
type affineTransformStruct struct {
	_                          structs.HostLayout
	m11, m12, m21, m22, tX, tY float64
}

func TestSendStruct(t *testing.T) {
	loadFoundation(t)
	selNew := objc.RegisterName("new")
	selRelease := objc.RegisterName("release")
	selSetTransformStruct := objc.RegisterName("setTransformStruct:")
	selTransformStruct := objc.RegisterName("transformStruct")

	transform := objc.ID(objc.GetClass("NSAffineTransform")).Send(selNew)
	defer transform.Send(selRelease)

	want := affineTransformStruct{m11: 1.5, m12: 2.5, m21: 3.5, m22: 4.5, tX: 5.5, tY: 6.5}
	objcutil.SendStruct(transform, selSetTransformStruct, unsafe.Pointer(&want), unsafe.Sizeof(want))
	if got := objc.Send[affineTransformStruct](transform, selTransformStruct); got != want {
		t.Errorf("transformStruct: got %+v, want %+v", got, want)
	}

	if n := testing.AllocsPerRun(100, func() {
		objcutil.SendStruct(transform, selSetTransformStruct, unsafe.Pointer(&want), unsafe.Sizeof(want))
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}
