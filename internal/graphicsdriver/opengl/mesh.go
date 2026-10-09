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

//go:build !playstation5

package opengl

import (
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver/opengl/gl"
)

// mesh is a vertex list and an index list on the GPU, with a vertex array that reads the vertex attributes
// from the vertex list and the instance attributes from the instance buffer.
type mesh struct {
	vertexArray  uint32
	vertexBuffer buffer
	indexBuffer  buffer
	indexCount   int32

	// instanceBuffer is the generation of the instance buffer that the instance attributes point at.
	// A buffer name is not enough, as a new buffer can take the name of a deleted one.
	instanceBuffer uint64
}

// meshDrawState is the state that the last DrawMesh leaves, so that the next DrawMesh does not set it again.
// A zero vertexArray means that the vertex array and the array buffer of the batches are bound.
type meshDrawState struct {
	vertexArray   uint32
	scissorWidth  int
	scissorHeight int
}

// setDepthTest turns the depth test on or off. Each draw sets it, so that it is off for a draw without depth.
func (g *Graphics) setDepthTest(on bool) {
	if g.state.depthTest == on {
		return
	}
	if on {
		g.context.ctx.Enable(gl.DEPTH_TEST)
		g.context.ctx.DepthFunc(gl.LEQUAL)
	} else {
		g.context.ctx.Disable(gl.DEPTH_TEST)
	}
	g.state.depthTest = on
}

// endMeshDraws binds the vertex array and the array buffer of the batches again, for DrawTriangles and SetVertices.
func (g *Graphics) endMeshDraws() {
	s := &g.state.meshDraw
	if s.vertexArray == 0 {
		return
	}
	g.context.ctx.BindVertexArray(g.state.vertexArray)
	g.context.ctx.BindBuffer(gl.ARRAY_BUFFER, uint32(g.state.arrayBuffer))
	*s = meshDrawState{}
}

// CanReadDepth implements graphicsdriver.DepthSourcer. Depth textures are core in OpenGL 3.2, OpenGL ES 3.0, and WebGL 2.
func (g *Graphics) CanReadDepth() bool {
	return true
}

func (g *Graphics) CanDrawMesh() bool {
	return g.context.ctx.HasInstancing()
}

func (g *Graphics) NewMesh(id graphicsdriver.MeshID, vertices []float32, indices []uint32) error {
	if len(vertices) == 0 || len(indices) == 0 {
		return nil
	}

	g.endMeshDraws()

	context := &g.context
	m := &mesh{
		indexCount: int32(len(indices)),
	}
	m.vertexArray = context.ctx.CreateVertexArray()
	context.ctx.BindVertexArray(m.vertexArray)

	m.vertexBuffer = buffer(context.ctx.CreateBuffer())
	context.ctx.BindBuffer(gl.ARRAY_BUFFER, uint32(m.vertexBuffer))
	vs := unsafe.Slice((*byte)(unsafe.Pointer(&vertices[0])), len(vertices)*int(unsafe.Sizeof(vertices[0])))
	context.ctx.BufferInit(gl.ARRAY_BUFFER, len(vs), gl.STATIC_DRAW)
	context.ctx.BufferSubData(gl.ARRAY_BUFFER, 0, vs)
	theArrayBufferLayout.enable(context)

	m.indexBuffer = buffer(context.ctx.CreateBuffer())
	context.ctx.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, uint32(m.indexBuffer))
	is := unsafe.Slice((*byte)(unsafe.Pointer(&indices[0])), len(indices)*int(unsafe.Sizeof(indices[0])))
	context.ctx.BufferInit(gl.ELEMENT_ARRAY_BUFFER, len(is), gl.STATIC_DRAW)
	context.ctx.BufferSubData(gl.ELEMENT_ARRAY_BUFFER, 0, is)

	// The instance attributes follow the vertex attributes, and advance once for each instance.
	n := len(theArrayBufferLayout.parts)
	for i := range n {
		context.ctx.EnableVertexAttribArray(uint32(n + i))
		context.ctx.VertexAttribDivisor(uint32(n+i), 1)
	}

	if g.state.meshes == nil {
		g.state.meshes = map[graphicsdriver.MeshID]*mesh{}
	}
	g.state.meshes[id] = m

	// DrawTriangles and SetVertices expect the vertex array and the array buffer of the batches.
	context.ctx.BindVertexArray(g.state.vertexArray)
	context.ctx.BindBuffer(gl.ARRAY_BUFFER, uint32(g.state.arrayBuffer))
	return nil
}

func (g *Graphics) DrawMesh(dstID graphicsdriver.ImageID, srcIDs [graphics.ShaderSrcImageCount]graphicsdriver.ImageID, shaderID graphicsdriver.ShaderID, meshID graphicsdriver.MeshID, instances []float32, blend graphicsdriver.Blend, uniforms []uint32, depth bool) error {
	// A mesh from before a context loss is unknown, and draws nothing.
	m, ok := g.state.meshes[meshID]
	if !ok || len(instances) == 0 {
		return nil
	}

	destination, err := g.useDestinationAndProgram(dstID, srcIDs, shaderID, blend, uniforms)
	if err != nil {
		return err
	}

	s := &g.state.meshDraw
	if s.vertexArray != m.vertexArray {
		g.context.ctx.BindVertexArray(m.vertexArray)
	}

	// The records of every draw go to offset 0 of one instance buffer, in a new store of their size. An upload
	// into the store that an earlier draw still reads makes the driver wait for that draw. The instance
	// attributes name the buffer, so they stay valid.
	is := unsafe.Slice((*byte)(unsafe.Pointer(&instances[0])), len(instances)*int(unsafe.Sizeof(instances[0])))
	if g.state.instanceBuffer == 0 {
		g.state.instanceBuffer = buffer(g.context.ctx.CreateBuffer())
		g.context.ctx.BindBuffer(gl.ARRAY_BUFFER, uint32(g.state.instanceBuffer))
		g.state.instanceBufferGeneration++
	} else if s.vertexArray == 0 {
		g.context.ctx.BindBuffer(gl.ARRAY_BUFFER, uint32(g.state.instanceBuffer))
	}
	g.context.ctx.BufferInit(gl.ARRAY_BUFFER, len(is), gl.STREAM_DRAW)
	g.context.ctx.BufferSubData(gl.ARRAY_BUFFER, 0, is)

	// Point the instance attributes of the vertex array at the instance buffer when the buffer is new to it.
	if m.instanceBuffer != g.state.instanceBufferGeneration {
		n := len(theArrayBufferLayout.parts)
		stride := int32(floatSizeInBytes * theArrayBufferLayout.float32Count())
		var offset int
		for i, p := range theArrayBufferLayout.parts {
			g.context.ctx.VertexAttribPointer(uint32(n+i), int32(p.num), gl.FLOAT, false, stride, offset)
			offset += floatSizeInBytes * p.num
		}
		m.instanceBuffer = g.state.instanceBufferGeneration
	}

	g.beginPass(destination, depth)
	if depth {
		if err := destination.useDepth(); err != nil {
			return err
		}
	}
	g.setDepthTest(depth)

	if s.vertexArray == 0 || s.scissorWidth != destination.width || s.scissorHeight != destination.height {
		g.context.ctx.Scissor(0, 0, int32(destination.width), int32(destination.height))
	}
	g.context.ctx.DrawElementsInstanced(gl.TRIANGLES, m.indexCount, gl.UNSIGNED_INT, 0, int32(len(instances)/graphics.VertexFloatCount))

	// DrawTriangles and SetVertices bind the vertex array and the array buffer of the batches again with endMeshDraws.
	*s = meshDrawState{
		vertexArray:   m.vertexArray,
		scissorWidth:  destination.width,
		scissorHeight: destination.height,
	}

	return nil
}

// DiscardDepth invalidates the depth attachment of the image on OpenGL ES, and binds the framebuffer of the image
// first when it is not bound, as after a read of its depth.
func (g *Graphics) DiscardDepth(id graphicsdriver.ImageID) {
	c, ok := g.context.ctx.(interface{ InvalidateDepth() })
	if i := g.images[id]; ok && g.context.ctx.IsES() && i != nil && i.depthTexture != 0 && i.framebuffer != nil {
		g.context.bindFramebuffer(i.framebuffer.native)
		c.InvalidateDepth()
	}
}

// useDepth gives the image a depth buffer on its first use, and clears the depth buffer on its first use in a frame.
// The framebuffer of the image must be bound.
func (i *Image) useDepth() error {
	c := &i.graphics.context
	clearDepth := i.depthFrame != i.graphics.frame
	if i.depthTexture == 0 {
		// A depth texture, not a renderbuffer, so that a draw can read it with Blend.SourceDepth. Making it binds it,
		// so bind the source of the draw on that unit again.
		src := c.lastTextures[c.lastActiveTexture]
		w, h := i.viewportSize()
		t, err := c.newTextureOfFormat(w, h, gl.DEPTH_COMPONENT24, gl.DEPTH_COMPONENT, gl.UNSIGNED_INT)
		if err != nil {
			return err
		}
		c.bindTexture(src)
		i.depthTexture = t
		c.ctx.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, uint32(t), 0)
		clearDepth = true
	}
	if clearDepth {
		// The scissor test would clip the clear.
		c.ctx.Disable(gl.SCISSOR_TEST)
		c.ctx.Clear(gl.DEPTH_BUFFER_BIT)
		c.ctx.Enable(gl.SCISSOR_TEST)
		i.depthFrame = i.graphics.frame
	}
	return nil
}
