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
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver/metal/mtl"
)

// TestDrawCalls checks the calls of a draw that send their messages without reflection:
// the viewport, the scissor rectangle, a vertex buffer at an offset, the vertex and fragment bytes,
// and the indexed draws. The viewport covers the left half of the target and the scissor rectangle
// covers the top half, so a triangle that covers the clip space colours the top-left quarter only.
//
// Run it with MTL_DEBUG_LAYER=1 as well, so that the validation layer checks the arguments.
func TestDrawCalls(t *testing.T) {
	device, err := mtl.CreateSystemDefaultDevice()
	if err != nil {
		t.Skip(err)
	}

	const source = `#include <metal_stdlib>

using namespace metal;

struct VertexOut {
	float4 position [[position]];
};

vertex VertexOut VertexShader(
	uint vid [[vertex_id]],
	uint iid [[instance_id]],
	const device float4* positions [[buffer(0)]],
	constant float4& offset [[buffer(1)]]
) {
	VertexOut out;
	out.position = positions[vid] + offset + float4(float(iid) * 8.0, 0, 0, 0);
	return out;
}

fragment float4 FragmentShader(VertexOut in [[stage_in]], constant float4& color [[buffer(0)]]) {
	return color;
}
`
	lib, err := device.NewLibraryWithSource(source, mtl.CompileOptions{})
	if err != nil {
		t.Fatal(err)
	}
	vs, err := lib.NewFunctionWithName("VertexShader")
	if err != nil {
		t.Fatal(err)
	}
	fs, err := lib.NewFunctionWithName("FragmentShader")
	if err != nil {
		t.Fatal(err)
	}
	var rpld mtl.RenderPipelineDescriptor
	rpld.VertexFunction = vs
	rpld.FragmentFunction = fs
	rpld.ColorAttachments[0].PixelFormat = mtl.PixelFormatRGBA8UNorm
	rpld.ColorAttachments[0].WriteMask = mtl.ColorWriteMaskAll
	rps, err := device.NewRenderPipelineStateWithDescriptor(rpld)
	if err != nil {
		t.Fatal(err)
	}

	// The positions start at a byte offset of 256 in the buffer, after a block of garbage.
	const offset = 256
	positions := [...]float32{
		-1, -1, 0, 1,
		3, -1, 0, 1,
		-1, 3, 0, 1,
	}
	vb, err := device.NewBufferWithLength(offset+unsafe.Sizeof(positions), mtl.ResourceStorageModeManaged)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := vb.Length(), uintptr(offset+unsafe.Sizeof(positions)); got != want {
		t.Errorf("Length: got %d, want %d", got, want)
	}
	garbage := make([]float32, offset/4)
	for i := range garbage {
		garbage[i] = 100
	}
	all := append(garbage, positions[:]...)
	vb.CopyToContents(unsafe.Pointer(&all[0]), uintptr(len(all))*4)

	indices := [...]uint32{0, 1, 2}
	ib, err := device.NewBufferWithBytes(unsafe.Pointer(&indices[0]), unsafe.Sizeof(indices), mtl.ResourceStorageModeManaged)
	if err != nil {
		t.Fatal(err)
	}

	const size = 8
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

	var rpd mtl.RenderPassDescriptor
	rpd.ColorAttachments[0].LoadAction = mtl.LoadActionClear
	rpd.ColorAttachments[0].StoreAction = mtl.StoreActionStore
	rpd.ColorAttachments[0].Texture = texture
	rce, err := cb.RenderCommandEncoderWithDescriptor(rpd)
	if err != nil {
		t.Fatal(err)
	}
	rce.SetRenderPipelineState(rps)
	rce.SetViewport(mtl.Viewport{Width: size / 2, Height: size, ZNear: 0, ZFar: 1})
	rce.SetScissorRect(mtl.ScissorRect{Width: size, Height: size / 2})
	rce.SetVertexBuffer(vb, offset, 0)
	zero := make([]float32, 4)
	rce.SetVertexBytes(unsafe.Pointer(&zero[0]), 16, 1)
	color := []float32{1, 0, 0, 1}
	rce.SetFragmentBytes(unsafe.Pointer(&color[0]), 16, 0)
	rce.DrawIndexedPrimitives(mtl.PrimitiveTypeTriangle, len(indices), mtl.IndexTypeUInt32, ib, 0)
	rce.EndEncoding()

	bce, err := cb.BlitCommandEncoder()
	if err != nil {
		t.Fatal(err)
	}
	bce.Synchronize(texture)
	bce.EndEncoding()

	cb.Commit()
	cb.WaitUntilCompleted()

	pixels := make([]byte, 4*size*size)
	if err := texture.GetBytes(pixels, 4*size, mtl.RegionMake2D(0, 0, size, size), 0); err != nil {
		t.Fatal(err)
	}
	for y := range size {
		for x := range size {
			got := pixels[4*(y*size+x) : 4*(y*size+x)+4]
			want := []byte{0, 0, 0, 0}
			if x < size/2 && y < size/2 {
				want = []byte{0xff, 0, 0, 0xff}
			}
			if string(got) != string(want) {
				t.Errorf("pixel (%d, %d): got %v, want %v", x, y, got, want)
			}
		}
	}
}
