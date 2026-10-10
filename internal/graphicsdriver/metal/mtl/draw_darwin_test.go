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

package mtl_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver/metal/mtl"
)

// TestCopyFromTexture checks a copy of a region between two textures, from an origin to another
// origin.
func TestCopyFromTexture(t *testing.T) {
	device, err := mtl.CreateSystemDefaultDevice()
	if err != nil {
		t.Skip(err)
	}

	const size = 4
	newTexture := func() mtl.Texture {
		texture, err := device.NewTextureWithDescriptor(mtl.TextureDescriptor{
			TextureType: mtl.TextureType2D,
			PixelFormat: mtl.PixelFormatRGBA8UNorm,
			Width:       size,
			Height:      size,
			StorageMode: mtl.StorageModeManaged,
			Usage:       mtl.TextureUsageShaderRead,
		})
		if err != nil {
			t.Fatal(err)
		}
		return texture
	}
	src, dst := newTexture(), newTexture()
	defer src.Release()
	defer dst.Release()
	pixels := make([]byte, 4*size*size)
	for i := range pixels {
		pixels[i] = byte(i)
	}
	// The buffer holds the pixels of src, then the zero pixels of dst.
	buf, err := device.NewBufferWithLength(uintptr(2*len(pixels)), mtl.ResourceStorageModeManaged)
	if err != nil {
		t.Fatal(err)
	}
	defer buf.Release()
	mtl.CopyToBufferAt(buf, pixels, 0)
	mtl.CopyToBufferAt(buf, make([]byte, len(pixels)), uintptr(len(pixels)))

	cq, err := device.NewCommandQueue()
	if err != nil {
		t.Fatal(err)
	}
	cb, err := cq.CommandBuffer()
	if err != nil {
		t.Fatal(err)
	}
	bce, err := cb.BlitCommandEncoder()
	if err != nil {
		t.Fatal(err)
	}
	whole := mtl.Size{Width: size, Height: size, Depth: 1}
	bce.CopyFromBuffer(buf, 0, 4*size, uintptr(len(pixels)), whole, src, 0, 0, mtl.Origin{})
	bce.CopyFromBuffer(buf, uintptr(len(pixels)), 4*size, uintptr(len(pixels)), whole, dst, 0, 0, mtl.Origin{})
	bce.EndEncoding()
	// The 2x2 block at (1, 0) of src goes to (2, 1) of dst.
	copyBlock := func() {
		bce, err := cb.BlitCommandEncoder()
		if err != nil {
			t.Fatal(err)
		}
		bce.CopyFromTexture(src, 0, 0, mtl.Origin{X: 1}, mtl.Size{Width: 2, Height: 2, Depth: 1}, dst, 0, 0, mtl.Origin{X: 2, Y: 1})
		bce.EndEncoding()
	}
	copyBlock()
	bce, err = cb.BlitCommandEncoder()
	if err != nil {
		t.Fatal(err)
	}
	bce.Synchronize(dst)
	bce.EndEncoding()
	cb.Commit()
	cb.WaitUntilCompleted()

	got := make([]byte, 4*size*size)
	if err := dst.GetBytes(got, 4*size, mtl.RegionMake2D(0, 0, size, size), 0); err != nil {
		t.Fatal(err)
	}
	for y := range size {
		for x := range size {
			want := []byte{0, 0, 0, 0}
			if x >= 2 && y >= 1 && y < 3 {
				i := 4 * ((y-1)*size + x - 1)
				want = pixels[i : i+4]
			}
			if p := got[4*(y*size+x) : 4*(y*size+x)+4]; string(p) != string(want) {
				t.Errorf("pixel (%d, %d): got %v, want %v", x, y, p, want)
			}
		}
	}
}

// TestGPUTimes checks the GPU start and end times of a completed command buffer.
func TestGPUTimes(t *testing.T) {
	device, err := mtl.CreateSystemDefaultDevice()
	if err != nil {
		t.Skip(err)
	}
	cq, err := device.NewCommandQueue()
	if err != nil {
		t.Fatal(err)
	}
	cb, err := cq.CommandBuffer()
	if err != nil {
		t.Fatal(err)
	}
	cb.Retain()
	defer cb.Release()
	cb.Commit()
	cb.WaitUntilCompleted()

	start, end := cb.GPUStartTime(), cb.GPUEndTime()
	if start <= 0 || end < start {
		t.Errorf("GPU times: got %v to %v, want a positive start and an end at or after it", start, end)
	}
}
