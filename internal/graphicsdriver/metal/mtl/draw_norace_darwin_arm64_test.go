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

package mtl_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver/metal/mtl"
)

// TestCopyFromTextureAllocs checks that a copy of a region between two textures does not allocate.
func TestCopyFromTextureAllocs(t *testing.T) {
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

	cq, err := device.NewCommandQueue()
	if err != nil {
		t.Fatal(err)
	}
	cb, err := cq.CommandBuffer()
	if err != nil {
		t.Fatal(err)
	}
	copyBlock := func() {
		bce, err := cb.BlitCommandEncoder()
		if err != nil {
			t.Fatal(err)
		}
		bce.CopyFromTexture(src, 0, 0, mtl.Origin{X: 1}, mtl.Size{Width: 2, Height: 2, Depth: 1}, dst, 0, 0, mtl.Origin{X: 2, Y: 1})
		bce.EndEncoding()
	}
	copyBlock()
	if n := testing.AllocsPerRun(10, copyBlock); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
	cb.Commit()
	cb.WaitUntilCompleted()
}
