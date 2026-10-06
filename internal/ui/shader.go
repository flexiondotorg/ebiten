// Copyright 2022 The Ebiten Authors
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
	"fmt"
	"math"
	"reflect"

	"github.com/hajimehoshi/ebiten/v2/internal/atlas"
	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/shaderir"
)

type Shader struct {
	shader *atlas.Shader

	uniformNames       []string
	uniformTypes       []shaderir.Type
	uniformDwordCounts []int
	uniformDwordCount  int
}

func NewShader(ir *shaderir.Program, name string) *Shader {
	uniformTypes := ir.Uniforms[graphics.PreservedUniformVariablesCount:]
	uniformDwordCounts := make([]int, len(uniformTypes))
	var uniformDwordCount int
	for i, typ := range uniformTypes {
		uniformDwordCounts[i] = typ.DwordCount()
		uniformDwordCount += uniformDwordCounts[i]
	}
	return &Shader{
		shader:             atlas.NewShader(ir, name),
		uniformNames:       ir.UniformNames[graphics.PreservedUniformVariablesCount:],
		uniformTypes:       uniformTypes,
		uniformDwordCounts: uniformDwordCounts,
		uniformDwordCount:  uniformDwordCount,
	}
}

func (s *Shader) Deallocate() {
	s.shader.Deallocate()
}

func (s *Shader) AppendUniforms(dst []uint32, uniforms map[string]any) []uint32 {
	origLen := len(dst)
	if cap(dst)-len(dst) >= s.uniformDwordCount {
		dst = dst[:len(dst)+s.uniformDwordCount]
		for i := origLen; i < len(dst); i++ {
			dst[i] = 0
		}
	} else {
		dst = append(dst, make([]uint32, s.uniformDwordCount)...)
	}

	idx := origLen
	for i, name := range s.uniformNames {
		n := s.uniformDwordCounts[i]

		// Ignore if an unused name is specified (#2710).
		uv, ok := uniforms[name]
		if !ok {
			idx += n
			continue
		}

		// Write the common types without reflection. Other types, named types included, take the path below.
		var done bool
		switch uv := uv.(type) {
		case float32:
			done = n == 1
			if done {
				dst[idx] = math.Float32bits(uv)
			}
		case int:
			done = n == 1
			if done {
				dst[idx] = uint32(uv)
			}
		case []float32:
			done = n == len(uv)
			if done {
				for j, f := range uv {
					dst[idx+j] = math.Float32bits(f)
				}
			}
		case []int:
			done = n == len(uv)
			if done {
				for j, v := range uv {
					dst[idx+j] = uint32(v)
				}
			}
		}
		if done {
			idx += n
			continue
		}

		typ := s.uniformTypes[i]
		v := reflect.ValueOf(uv)
		t := v.Type()
		switch t.Kind() {
		case reflect.Bool:
			if typ.DwordCount() != 1 {
				panic(fmt.Sprintf("ui: unexpected uniform value for %s (%s)", name, typ.String()))
			}
			if v.Bool() {
				dst[idx] = 1
			} else {
				dst[idx] = 0
			}
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if typ.DwordCount() != 1 {
				panic(fmt.Sprintf("ui: unexpected uniform value for %s (%s)", name, typ.String()))
			}
			dst[idx] = uint32(v.Int())
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			if typ.DwordCount() != 1 {
				panic(fmt.Sprintf("ui: unexpected uniform value for %s (%s)", name, typ.String()))
			}
			dst[idx] = uint32(v.Uint())
		case reflect.Float32, reflect.Float64:
			if typ.DwordCount() != 1 {
				panic(fmt.Sprintf("ui: unexpected uniform value for %s (%s)", name, typ.String()))
			}
			dst[idx] = math.Float32bits(float32(v.Float()))
		case reflect.Slice, reflect.Array:
			l := v.Len()
			if typ.DwordCount() != l {
				panic(fmt.Sprintf("ui: unexpected uniform value for %s (%s)", name, typ.String()))
			}
			switch t.Elem().Kind() {
			case reflect.Bool:
				for i := range l {
					if v.Index(i).Bool() {
						dst[idx+i] = 1
					} else {
						dst[idx+i] = 0
					}
				}
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				for i := range l {
					dst[idx+i] = uint32(v.Index(i).Int())
				}
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
				for i := range l {
					dst[idx+i] = uint32(v.Index(i).Uint())
				}
			case reflect.Float32, reflect.Float64:
				for i := range l {
					dst[idx+i] = math.Float32bits(float32(v.Index(i).Float()))
				}
			default:
				panic(fmt.Sprintf("ui: unexpected uniform value type: %s (%s)", name, v.Kind().String()))
			}
		default:
			panic(fmt.Sprintf("ui: unexpected uniform value type: %s (%s)", name, v.Kind().String()))
		}

		idx += n
	}

	return dst
}
