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

package metal

import "testing"

func TestInstanceSpace(t *testing.T) {
	var s instanceSpace
	place := func(frame int64, size, wantOffset, wantNewLength uintptr) {
		t.Helper()
		offset, newLength := s.place(frame, size)
		if offset != wantOffset || newLength != wantNewLength {
			t.Fatalf("place(%d, %d): got (%d, %d), want (%d, %d)", frame, size, offset, newLength, wantOffset, wantNewLength)
		}
		if newLength != 0 {
			// The pool rounds a new buffer up to a power of two.
			s.use(pow2(newLength), size)
		}
	}

	// The first frame starts without a buffer, and grows when the records do not fit.
	place(0, 48, 0, 48)
	place(0, 96, 0, 256+96)
	place(0, 48, 256, 0)
	place(0, 480, 0, 768+480)
	// The next frame asks for the bytes of the whole last frame, and then fits in one buffer.
	place(1, 48, 0, 768+480)
	place(1, 96, 256, 0)
	place(1, 48, 512, 0)
	place(1, 480, 768, 0)
	// A frame without records keeps the size of the last frame with records.
	place(3, 48, 0, 768+480)
	// Each offset is aligned.
	for i := range 20 {
		offset, newLength := s.place(4, uintptr(12*i+4))
		if newLength != 0 {
			s.use(pow2(newLength), uintptr(12*i+4))
			continue
		}
		if offset%instanceOffsetAlignment != 0 {
			t.Errorf("offset %d is not aligned", offset)
		}
		if offset+uintptr(12*i+4) > s.length {
			t.Errorf("records at %d end beyond the buffer of %d bytes", offset, s.length)
		}
	}
}

func TestInstanceSpaceAllocations(t *testing.T) {
	var s instanceSpace
	frame := int64(0)
	if n := testing.AllocsPerRun(100, func() {
		for range 300 {
			if _, newLength := s.place(frame, 48); newLength != 0 {
				s.use(pow2(newLength), 48)
			}
		}
		frame++
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
}
