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

package ui

import (
	"slices"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/shaderir"
)

type (
	namedFloat32 float32
	namedInt     int
)

func newTestShader() *Shader {
	ir := &shaderir.Program{}
	for range graphics.PreservedUniformVariablesCount {
		ir.UniformNames = append(ir.UniformNames, "")
		ir.Uniforms = append(ir.Uniforms, shaderir.Type{Main: shaderir.Float})
	}
	ir.UniformNames = append(ir.UniformNames, "F", "I", "V", "A", "Unused")
	ir.Uniforms = append(ir.Uniforms,
		shaderir.Type{Main: shaderir.Float},
		shaderir.Type{Main: shaderir.Int},
		shaderir.Type{Main: shaderir.Vec4},
		shaderir.Type{Main: shaderir.Array, Length: 3, Sub: []shaderir.Type{{Main: shaderir.Int}}},
		shaderir.Type{Main: shaderir.Float},
	)
	return NewShader(ir, "test")
}

// TestAppendUniformsFastTypes checks that the types without reflection give the same values as the named types,
// which take the reflection path.
func TestAppendUniformsFastTypes(t *testing.T) {
	s := newTestShader()
	fast := map[string]any{
		"F": float32(1.5),
		"I": -2,
		"V": []float32{1, -2, 3.25, 4},
		"A": []int{5, -6, 7},
	}
	named := map[string]any{
		"F": namedFloat32(1.5),
		"I": namedInt(-2),
		"V": []namedFloat32{1, -2, 3.25, 4},
		"A": []namedInt{5, -6, 7},
	}
	got := s.AppendUniforms([]uint32{9}, fast)
	want := s.AppendUniforms([]uint32{9}, named)
	if !slices.Equal(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}

	dst := make([]uint32, 0, 64)
	if n := testing.AllocsPerRun(100, func() { dst = s.AppendUniforms(dst[:0], fast) }); n != 0 {
		t.Errorf("AppendUniforms allocated %v times, want 0", n)
	}
}

func TestAppendUniformsWrongLength(t *testing.T) {
	s := newTestShader()
	defer func() {
		if recover() == nil {
			t.Errorf("AppendUniforms with a slice of the wrong length must panic but does not")
		}
	}()
	s.AppendUniforms(nil, map[string]any{"V": []float32{1, 2, 3}})
}
