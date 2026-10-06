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

import "math"

// instanceOffsetAlignment is the alignment of the byte offset of the instance records of a mesh
// draw in the instance buffer. setVertexBuffer:offset:atIndex: needs 256 bytes for a buffer in the
// constant address space on macOS, and less for the device address space of the instance records.
// 256 bytes is safe for both.
const instanceOffsetAlignment = 256

// instanceSpace places the instance records of the mesh draws of a frame in one instance buffer
// for each frame, in place of a new Metal buffer for each draw.
type instanceSpace struct {
	// frame is the frame of the current buffer.
	frame int64

	// length is the length of the current buffer, or 0 when the frame has no buffer yet.
	length uintptr

	// offset is the end of the records in the current buffer.
	offset uintptr

	// frameBytes is the bytes that the records of frame need in one buffer.
	frameBytes uintptr

	// lastFrameBytes is the bytes that the records of the last frame with records needed in one buffer.
	lastFrameBytes uintptr
}

func pow2(x uintptr) uintptr {
	if x > (math.MaxUint+1)/2 {
		return math.MaxUint
	}

	var p2 uintptr = 1
	for p2 < x {
		p2 *= 2
	}
	return p2
}

func alignUp(x, alignment uintptr) uintptr {
	return (x + alignment - 1) &^ (alignment - 1)
}

// place returns the offset of size bytes of records in the current buffer of frame. When the
// records do not fit, place returns a newLength that is not zero: the caller must get a buffer of at
// least newLength bytes, call use with its length, and put the records at offset 0.
func (s *instanceSpace) place(frame int64, size uintptr) (offset, newLength uintptr) {
	if frame != s.frame {
		// The buffer of an earlier frame can be in use by the GPU until that frame completes.
		if s.frameBytes > 0 {
			s.lastFrameBytes = s.frameBytes
		}
		s.frame = frame
		s.length = 0
		s.offset = 0
		s.frameBytes = 0
	}
	s.frameBytes = alignUp(s.frameBytes, instanceOffsetAlignment) + size

	offset = alignUp(s.offset, instanceOffsetAlignment)
	if s.length != 0 && offset+size <= s.length {
		s.offset = offset + size
		return offset, 0
	}
	// Ask for the whole frame at once, so that the next frame uses one buffer.
	return 0, max(size, s.lastFrameBytes, s.frameBytes, 2*s.length)
}

// use records a new buffer of length bytes, with the first size bytes of records at offset 0.
func (s *instanceSpace) use(length, size uintptr) {
	s.length = length
	s.offset = size
}
