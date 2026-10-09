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

package ebiten

import (
	"fmt"
	"slices"
	"unsafe"

	"github.com/hajimehoshi/ebiten/v2/internal/atlas"
	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/graphicscommand"
	"github.com/hajimehoshi/ebiten/v2/internal/ui"
)

// Mesh is a vertex list and an index list that stay on the GPU. Draw it with
// DrawTrianglesShader32 and DrawTrianglesShaderOptions.Mesh. A context loss (Android)
// leaves a mesh invalid, and a draw with an invalid mesh does nothing.
type Mesh struct {
	mesh *graphicscommand.Mesh
}

// NewMesh uploads the vertices and the indices. It panics when IsMeshDrawingSupported
// is false. Call it after the game starts.
//
// If len(indices) is not multiple of 3, or a value in indices is out of range of vertices, NewMesh panics.
func NewMesh(vertices []Vertex, indices []uint32) *Mesh {
	if !IsMeshDrawingSupported() {
		panic("ebiten: the graphics driver cannot draw meshes")
	}
	if len(indices)%3 != 0 {
		panic("ebiten: len(indices) % 3 must be 0")
	}
	for i, idx := range indices {
		if idx >= uint32(len(vertices)) {
			panic(fmt.Sprintf("ebiten: indices[%d] must be less than len(vertices) (%d) but was %d", i, len(vertices), idx))
		}
	}
	// Vertex has the same layout as the internal vertex format. The copies live until the upload.
	vs := slices.Clone(unsafe.Slice((*float32)(unsafe.Pointer(unsafe.SliceData(vertices))), len(vertices)*graphics.VertexFloatCount))
	return &Mesh{mesh: atlas.NewMesh(vs, slices.Clone(indices))}
}

// IsMeshDrawingSupported reports whether the graphics driver can draw a Mesh.
func IsMeshDrawingSupported() bool {
	return ui.Get().IsMeshDrawingSupported()
}

// IsDepthSourceSupported reports whether a draw can read the depth buffer of an image with
// DrawTrianglesShaderOptions.SourceDepth. It is true on OpenGL, OpenGL ES, and WebGL 2. Call it
// after the game starts.
func IsDepthSourceSupported() bool {
	return ui.Get().IsDepthSourceSupported()
}
