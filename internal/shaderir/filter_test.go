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

package shaderir_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
)

func TestFilterUniformVariables(t *testing.T) {
	src := []byte(`//kage:unit pixels

package main

var Used float
var Unused1 vec2
var Unused2 mat4
var Used2 [3]float

func Fragment(dst vec4, src vec2, color vec4) vec4 {
	return vec4(Used, Used2[1], 0, 1)
}
`)
	p, err := graphics.CompileShader(src)
	if err != nil {
		t.Fatal(err)
	}

	// The expected dwords, from the reachable uniform variables.
	reachable := map[int]bool{}
	for _, i := range p.AppendReachableUniformVariablesFromBlock(nil, p.VertexFunc.Block) {
		reachable[i] = true
	}
	for _, i := range p.AppendReachableUniformVariablesFromBlock(nil, p.FragmentFunc.Block) {
		reachable[i] = true
	}
	var want []uint32
	for i, typ := range p.Uniforms {
		for range typ.DwordCount() {
			v := uint32(len(want) + 1)
			if !reachable[i] {
				v = 0
			}
			want = append(want, v)
		}
	}

	for range 2 {
		uniforms := make([]uint32, len(want))
		for i := range uniforms {
			uniforms[i] = uint32(i + 1)
		}
		p.FilterUniformVariables(uniforms)
		for i := range want {
			if uniforms[i] != want[i] {
				t.Fatalf("dword %d: got %d, want %d", i, uniforms[i], want[i])
			}
		}
	}

	// The last three uniform variables add 1 + 2 + 16 + 3 dwords. Only Used and Used2 stay.
	n := len(want)
	if got := want[n-22:]; got[0] == 0 || got[1] != 0 || got[18] != 0 || got[19] == 0 || got[21] == 0 {
		t.Errorf("the dwords of the variables of the test: %v", got)
	}
}
