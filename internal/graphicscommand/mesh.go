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

package graphicscommand

import (
	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver"
)

var nextMeshID graphicsdriver.MeshID = 1

// Mesh is a vertex list and an index list that the graphics driver keeps on the GPU.
type Mesh struct {
	id graphicsdriver.MeshID
}

// NewMesh enqueues the upload of the vertices and the indices, and keeps the slices until the flush.
func NewMesh(vertices []float32, indices []uint32) *Mesh {
	m := &Mesh{
		id: nextMeshID,
	}
	nextMeshID++
	theCommandQueueManager.enqueueCommand(&newMeshCommand{
		mesh:     m,
		vertices: vertices,
		indices:  indices,
	})
	return m
}
