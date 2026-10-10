// Copyright 2023 The Ebitengine Authors
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

package directx

import (
	"errors"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
)

type image12 struct {
	graphics *graphics12
	id       graphicsdriver.ImageID
	width    int
	height   int
	screen   bool

	states            [frameCount]_D3D12_RESOURCE_STATES
	texture           *_ID3D12Resource
	rtvDescriptorHeap *_ID3D12DescriptorHeap

	uploadingStagingBuffers []*_ID3D12Resource

	// depthTexture and dsvDescriptorHeap are the depth buffer, and depthFrame is the frame in which it was cleared last.
	// dsv writes the depth, and dsvReadOnly does not, so that a draw can test the depth and read it in a shader too.
	// depthState is the state of the depth buffer.
	depthTexture      *_ID3D12Resource
	dsvDescriptorHeap *_ID3D12DescriptorHeap
	depthFrame        int64
	dsv               _D3D12_CPU_DESCRIPTOR_HANDLE
	dsvReadOnly       _D3D12_CPU_DESCRIPTOR_HANDLE
	depthState        _D3D12_RESOURCE_STATES
}

func (i *image12) ID() graphicsdriver.ImageID {
	return i.id
}

func (i *image12) Dispose() {
	// Dispose the image later as this image might still be used.
	i.graphics.removeImage(i)
}

func (i *image12) disposeImpl() {
	i.releaseUploadingStagingBuffers()
	if i.rtvDescriptorHeap != nil {
		i.rtvDescriptorHeap.Release()
		i.rtvDescriptorHeap = nil
	}
	if i.texture != nil {
		i.texture.Release()
		i.texture = nil
	}
	if i.dsvDescriptorHeap != nil {
		i.dsvDescriptorHeap.Release()
		i.dsvDescriptorHeap = nil
	}
	if i.depthTexture != nil {
		i.depthTexture.Release()
		i.depthTexture = nil
	}
}

func (i *image12) WritePixels(args []graphicsdriver.PixelsArgs) error {
	if i.screen {
		return errors.New("directx: WritePixels cannot be called on the screen")
	}

	if err := i.graphics.flushCommandList(i.graphics.drawCommandList); err != nil {
		return err
	}

	// Place each region's pixels in its own footprint in one staging buffer, so that the buffer size is the
	// total size of the regions regardless of how far apart the regions are.
	layouts := make([]_D3D12_PLACED_SUBRESOURCE_FOOTPRINT, 0, len(args))
	var totalBytes uint64
	for _, a := range args {
		desc := _D3D12_RESOURCE_DESC{
			Dimension:        _D3D12_RESOURCE_DIMENSION_TEXTURE2D,
			Alignment:        0,
			Width:            uint64(a.Region.Dx()),
			Height:           uint32(a.Region.Dy()),
			DepthOrArraySize: 1,
			MipLevels:        0,
			Format:           _DXGI_FORMAT_R8G8B8A8_UNORM,
			SampleDesc: _DXGI_SAMPLE_DESC{
				Count:   1,
				Quality: 0,
			},
			Layout: _D3D12_TEXTURE_LAYOUT_UNKNOWN,
			Flags:  _D3D12_RESOURCE_FLAG_ALLOW_RENDER_TARGET,
		}
		// A placed footprint must start at a multiple of the placement alignment.
		offset := (totalBytes + _D3D12_TEXTURE_DATA_PLACEMENT_ALIGNMENT - 1) &^ (_D3D12_TEXTURE_DATA_PLACEMENT_ALIGNMENT - 1)
		layout, _, _, n := i.graphics.device.GetCopyableFootprints(&desc, 0, 1, offset)
		layouts = append(layouts, layout)
		totalBytes = layout.Offset + n
	}

	uploadingStagingBuffer, err := createBuffer(i.graphics.device, totalBytes, _D3D12_HEAP_TYPE_UPLOAD)
	if err != nil {
		return err
	}
	i.uploadingStagingBuffers = append(i.uploadingStagingBuffers, uploadingStagingBuffer)

	if rb, ok := i.transiteState(_D3D12_RESOURCE_STATE_COPY_DEST); ok {
		i.graphics.copyCommandList.ResourceBarrier([]_D3D12_RESOURCE_BARRIER_Transition{rb})
	}

	m, err := uploadingStagingBuffer.Map(0, &_D3D12_RANGE{Begin: 0, End: 0})
	if err != nil {
		return err
	}

	i.graphics.needFlushCopyCommandList = true

	srcBytes := unsafe.Slice((*byte)(unsafe.Pointer(m)), totalBytes)
	for idx, a := range args {
		layout := layouts[idx]
		w := 4 * a.Region.Dx()
		for j := 0; j < a.Region.Dy(); j++ {
			copy(srcBytes[layout.Offset+uint64(j)*uint64(layout.Footprint.RowPitch):], a.Pixels[j*w:(j+1)*w])
		}
	}

	for idx, a := range args {
		dst := _D3D12_TEXTURE_COPY_LOCATION_SubresourceIndex{
			pResource:        i.texture,
			Type:             _D3D12_TEXTURE_COPY_TYPE_SUBRESOURCE_INDEX,
			SubresourceIndex: 0,
		}
		src := _D3D12_TEXTURE_COPY_LOCATION_PlacedFootprint{
			pResource:       uploadingStagingBuffer,
			Type:            _D3D12_TEXTURE_COPY_TYPE_PLACED_FOOTPRINT,
			PlacedFootprint: layouts[idx],
		}
		i.graphics.copyCommandList.CopyTextureRegion_SubresourceIndex_PlacedFootprint(
			&dst, uint32(a.Region.Min.X), uint32(a.Region.Min.Y), 0, &src, &_D3D12_BOX{
				left:   0,
				top:    0,
				front:  0,
				right:  uint32(a.Region.Dx()),
				bottom: uint32(a.Region.Dy()),
				back:   1,
			})
	}

	uploadingStagingBuffer.Unmap(0, nil)

	return nil
}

func (i *image12) resource() *_ID3D12Resource {
	if i.screen {
		return i.graphics.renderTargets[i.graphics.backBufferIndex]
	}
	return i.texture
}

func (i *image12) state() _D3D12_RESOURCE_STATES {
	if i.screen {
		return i.states[i.graphics.backBufferIndex]
	}
	return i.states[0]
}

func (i *image12) setState(newState _D3D12_RESOURCE_STATES) {
	if i.screen {
		i.states[i.graphics.backBufferIndex] = newState
		return
	}
	i.states[0] = newState
}

func (i *image12) transiteState(newState _D3D12_RESOURCE_STATES) (_D3D12_RESOURCE_BARRIER_Transition, bool) {
	if i.state() == newState {
		return _D3D12_RESOURCE_BARRIER_Transition{}, false
	}
	oldState := i.state()
	i.setState(newState)

	return _D3D12_RESOURCE_BARRIER_Transition{
		Type:  _D3D12_RESOURCE_BARRIER_TYPE_TRANSITION,
		Flags: _D3D12_RESOURCE_BARRIER_FLAG_NONE,
		Transition: _D3D12_RESOURCE_TRANSITION_BARRIER{
			pResource:   i.resource(),
			Subresource: _D3D12_RESOURCE_BARRIER_ALL_SUBRESOURCES,
			StateBefore: oldState,
			StateAfter:  newState,
		},
	}, true
}

func (i *image12) internalSize() (int, int) {
	if i.screen {
		return i.width, i.height
	}
	return graphics.InternalImageSize(i.width), graphics.InternalImageSize(i.height)
}

// setAsRenderTarget sets i as the render target, with the depth-stencil view dsv if it is not nil.
func (i *image12) setAsRenderTarget(drawCommandList *_ID3D12GraphicsCommandList, device *_ID3D12Device, dsv *_D3D12_CPU_DESCRIPTOR_HANDLE) error {
	if err := i.ensureRenderTargetView(device); err != nil {
		return err
	}

	if i.screen {
		rtv, err := i.graphics.rtvDescriptorHeap.GetCPUDescriptorHandleForHeapStart()
		if err != nil {
			return err
		}
		rtv.Offset(int32(i.graphics.backBufferIndex), i.graphics.rtvDescriptorSize)
		drawCommandList.OMSetRenderTargets([]_D3D12_CPU_DESCRIPTOR_HANDLE{rtv}, false, dsv)
		return nil
	}

	rtv, err := i.rtvDescriptorHeap.GetCPUDescriptorHandleForHeapStart()
	if err != nil {
		return err
	}

	drawCommandList.OMSetRenderTargets([]_D3D12_CPU_DESCRIPTOR_HANDLE{rtv}, false, dsv)
	return nil
}

// ensureDepthStencilView makes the depth buffer of i and its two views on its first use. It is untested.
//
// The resource is typeless, so that the depth-stencil views write it as D32_FLOAT and a shader resource view reads it
// as R32_FLOAT. The depth has no stencil, as no draw uses one.
func (i *image12) ensureDepthStencilView(device *_ID3D12Device) error {
	if i.dsvDescriptorHeap != nil {
		return nil
	}

	w, h := i.internalSize()
	t, err := device.CreateCommittedResource(&_D3D12_HEAP_PROPERTIES{
		Type:                 _D3D12_HEAP_TYPE_DEFAULT,
		CPUPageProperty:      _D3D12_CPU_PAGE_PROPERTY_UNKNOWN,
		MemoryPoolPreference: _D3D12_MEMORY_POOL_UNKNOWN,
		CreationNodeMask:     1,
		VisibleNodeMask:      1,
	}, _D3D12_HEAP_FLAG_NONE, &_D3D12_RESOURCE_DESC{
		Dimension:        _D3D12_RESOURCE_DIMENSION_TEXTURE2D,
		Width:            uint64(w),
		Height:           uint32(h),
		DepthOrArraySize: 1,
		MipLevels:        1,
		Format:           _DXGI_FORMAT_R32_TYPELESS,
		SampleDesc:       _DXGI_SAMPLE_DESC{Count: 1},
		Layout:           _D3D12_TEXTURE_LAYOUT_UNKNOWN,
		Flags:            _D3D12_RESOURCE_FLAG_ALLOW_DEPTH_STENCIL,
	}, _D3D12_RESOURCE_STATE_DEPTH_WRITE, &_D3D12_CLEAR_VALUE{
		Format: _DXGI_FORMAT_D32_FLOAT,
		Color:  [4]float32{1}, // The depth of the union D3D12_DEPTH_STENCIL_VALUE.
	})
	if err != nil {
		return err
	}

	heap, err := device.CreateDescriptorHeap(&_D3D12_DESCRIPTOR_HEAP_DESC{
		Type:           _D3D12_DESCRIPTOR_HEAP_TYPE_DSV,
		NumDescriptors: 2,
		Flags:          _D3D12_DESCRIPTOR_HEAP_FLAG_NONE,
	})
	if err != nil {
		t.Release()
		return err
	}

	dsv, err := heap.GetCPUDescriptorHandleForHeapStart()
	if err != nil {
		heap.Release()
		t.Release()
		return err
	}
	desc := _D3D12_DEPTH_STENCIL_VIEW_DESC{
		Format:        _DXGI_FORMAT_D32_FLOAT,
		ViewDimension: _D3D12_DSV_DIMENSION_TEXTURE2D,
	}
	device.CreateDepthStencilView(t, &desc, dsv)
	dsvReadOnly := dsv
	dsvReadOnly.Offset(1, device.GetDescriptorHandleIncrementSize(_D3D12_DESCRIPTOR_HEAP_TYPE_DSV))
	desc.Flags = _D3D12_DSV_FLAG_READ_ONLY_DEPTH
	device.CreateDepthStencilView(t, &desc, dsvReadOnly)

	i.depthTexture = t
	i.dsvDescriptorHeap = heap
	i.dsv = dsv
	i.dsvReadOnly = dsvReadOnly
	i.depthState = _D3D12_RESOURCE_STATE_DEPTH_WRITE
	return nil
}

// transiteDepthState moves the depth buffer to newState, and returns the barrier if it is not there yet.
func (i *image12) transiteDepthState(newState _D3D12_RESOURCE_STATES) (_D3D12_RESOURCE_BARRIER_Transition, bool) {
	if i.depthState == newState {
		return _D3D12_RESOURCE_BARRIER_Transition{}, false
	}
	oldState := i.depthState
	i.depthState = newState
	return _D3D12_RESOURCE_BARRIER_Transition{
		Type:  _D3D12_RESOURCE_BARRIER_TYPE_TRANSITION,
		Flags: _D3D12_RESOURCE_BARRIER_FLAG_NONE,
		Transition: _D3D12_RESOURCE_TRANSITION_BARRIER{
			pResource:   i.depthTexture,
			Subresource: _D3D12_RESOURCE_BARRIER_ALL_SUBRESOURCES,
			StateBefore: oldState,
			StateAfter:  newState,
		},
	}, true
}

func (i *image12) ensureRenderTargetView(device *_ID3D12Device) error {
	if i.screen {
		return nil
	}

	if i.rtvDescriptorHeap != nil {
		return nil
	}

	h, err := device.CreateDescriptorHeap(&_D3D12_DESCRIPTOR_HEAP_DESC{
		Type:           _D3D12_DESCRIPTOR_HEAP_TYPE_RTV,
		NumDescriptors: 1,
		Flags:          _D3D12_DESCRIPTOR_HEAP_FLAG_NONE,
		NodeMask:       0,
	})
	if err != nil {
		return err
	}
	i.rtvDescriptorHeap = h

	rtv, err := i.rtvDescriptorHeap.GetCPUDescriptorHandleForHeapStart()
	if err != nil {
		return err
	}
	device.CreateRenderTargetView(i.texture, nil, rtv)

	return nil
}

func (i *image12) releaseUploadingStagingBuffers() {
	for idx, buf := range i.uploadingStagingBuffers {
		buf.Release()
		i.uploadingStagingBuffers[idx] = nil
	}
	i.uploadingStagingBuffers = i.uploadingStagingBuffers[:0]
}
