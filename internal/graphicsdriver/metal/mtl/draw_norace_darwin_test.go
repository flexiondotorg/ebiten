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

// TestRenderPassDescriptorCacheAllocs checks that a pass with a cached descriptor and an unchanged clear color does
// not allocate.
func TestRenderPassDescriptorCacheAllocs(t *testing.T) {
	device, err := mtl.CreateSystemDefaultDevice()
	if err != nil {
		t.Skip(err)
	}

	const size = 4
	texture, err := device.NewTextureWithDescriptor(mtl.TextureDescriptor{
		TextureType: mtl.TextureType2D,
		PixelFormat: mtl.PixelFormatRGBA8UNorm,
		Width:       size,
		Height:      size,
		StorageMode: mtl.StorageModeManaged,
		Usage:       mtl.TextureUsageShaderRead | mtl.TextureUsageRenderTarget,
	})
	if err != nil {
		t.Fatal(err)
	}
	cq, err := device.NewCommandQueue()
	if err != nil {
		t.Fatal(err)
	}
	cb, err := cq.CommandBuffer()
	if err != nil {
		t.Fatal(err)
	}

	var c mtl.RenderPassDescriptorCache
	defer c.Release()
	pass := func(load mtl.LoadAction, clear mtl.ClearColor) {
		var rpd mtl.RenderPassDescriptor
		rpd.ColorAttachments[0].LoadAction = load
		rpd.ColorAttachments[0].StoreAction = mtl.StoreActionStore
		rpd.ColorAttachments[0].ClearColor = clear
		rpd.ColorAttachments[0].Texture = texture
		rpd.RenderTargetWidth = size
		rpd.RenderTargetHeight = size
		rce, err := cb.RenderCommandEncoderWithDescriptorCache(&c, rpd)
		if err != nil {
			t.Fatal(err)
		}
		rce.EndEncoding()
	}
	pass(mtl.LoadActionClear, mtl.ClearColor{Red: 1, Alpha: 1})
	if n := testing.AllocsPerRun(10, func() {
		pass(mtl.LoadActionLoad, mtl.ClearColor{Red: 1, Alpha: 1})
	}); n != 0 {
		t.Errorf("allocations: got %v, want 0", n)
	}
	cb.Commit()
	cb.WaitUntilCompleted()
}
