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

package graphics_test

import (
	"bufio"
	"fmt"
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/shaderir/glsl"
)

func TestCompileShaderUnitDirective(t *testing.T) {
	const fragment = `package main

func Fragment(dstPos vec4, src0Pos vec2, color vec4) vec4 {
	return dstPos
}`
	cases := []struct {
		name string
		src  string
		err  bool
	}{
		{
			name: "no directive",
			src:  fragment,
			err:  true,
		},
		{
			name: "texels",
			src:  "//kage:unit texels\n\n" + fragment,
			err:  true,
		},
		{
			name: "invalid value",
			src:  "//kage:unit foo\n\n" + fragment,
			err:  true,
		},
		{
			name: "pixels",
			src:  "//kage:unit pixels\n\n" + fragment,
			err:  false,
		},
		{
			name: "duplicated",
			src:  "//kage:unit pixels\n//kage:unit pixels\n\n" + fragment,
			err:  true,
		},
		{
			name: "pixels after a long line",
			src:  "// " + strings.Repeat("a", bufio.MaxScanTokenSize) + "\n//kage:unit pixels\n\n" + fragment,
			err:  false,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := graphics.CompileShader([]byte(c.src))
			if err == nil && c.err {
				t.Errorf("CompileShader must return an error but does not")
			} else if err != nil && !c.err {
				t.Errorf("CompileShader must not return an error but returned %v", err)
			}
		})
	}
}

const userVertexShaderSource = `//kage:unit pixels

package main

var Model mat4

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4) (vec4, vec2, vec4, vec4) {
	pos := Model * vec4(dstPos, 0, 1)
	return imageDstProjection() * vec4(pos.xy+imageDstOrigin(), 0, 1), srcPos + imageSrc0Origin(), color, custom
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	return color
}
`

func TestCompileShaderUserVertex(t *testing.T) {
	ir, err := graphics.CompileShader([]byte(userVertexShaderSource))
	if err != nil {
		t.Fatal(err)
	}
	if !ir.UserVertex {
		t.Errorf("ir.UserVertex: got: false, want: true")
	}
	// Model is the first uniform after the preserved ones.
	model := fmt.Sprintf("U%d", graphics.PreservedUniformVariablesCount)
	for _, version := range []glsl.GLSLVersion{glsl.GLSLVersionDefault, glsl.GLSLVersionES300} {
		vs, _ := glsl.Compile(ir, version)
		if !strings.Contains(vs, "uniform mat4 "+model+";") || !strings.Contains(vs, model+") * (vec4(") {
			t.Errorf("version %d: the vertex shader must use %s but does not:\n%s", version, model, vs)
		}
	}
}

func TestCompileShaderUserVertexSignature(t *testing.T) {
	cases := []struct {
		name   string
		vertex string
	}{
		{
			name:   "fewer attributes",
			vertex: "func Vertex(dstPos vec2, srcPos vec2, color vec4) (vec4, vec2, vec4, vec4) {\n\treturn vec4(dstPos, 0, 1), srcPos, color, color\n}",
		},
		{
			name:   "wrong attribute type",
			vertex: "func Vertex(dstPos vec4, srcPos vec2, color vec4, custom vec4) (vec4, vec2, vec4, vec4) {\n\treturn dstPos, srcPos, color, custom\n}",
		},
		{
			name:   "fewer varyings",
			vertex: "func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4) (vec4, vec2, vec4) {\n\treturn vec4(dstPos, 0, 1), srcPos, color\n}",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			src := "//kage:unit pixels\n\npackage main\n\n" + c.vertex + "\n\nfunc Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {\n\treturn color\n}\n"
			if _, err := graphics.CompileShader([]byte(src)); err == nil {
				t.Errorf("CompileShader must return an error but does not")
			}
		})
	}
}

func TestCompileShaderBuiltinVertex(t *testing.T) {
	const src = `//kage:unit pixels

package main

// func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4) (vec4, vec2, vec4, vec4)

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	return color
}
`
	ir, err := graphics.CompileShader([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if ir.UserVertex {
		t.Errorf("ir.UserVertex: got: true, want: false")
	}
	// The complete source, and then the program, must be the same as in v2.10.4.
	if got, want := graphics.CalcSourceID([]byte(src)).String(), "joozshohbaxb53d5o5joudhkre"; got != want {
		t.Errorf("graphics.CalcSourceID: got: %s, want: %s", got, want)
	}
}
