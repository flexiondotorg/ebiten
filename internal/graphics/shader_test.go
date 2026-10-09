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
	"github.com/hajimehoshi/ebiten/v2/internal/shaderir/hlsl"
	"github.com/hajimehoshi/ebiten/v2/internal/shaderir/msl"
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

// TestCompileShaderUserVertexClipZ checks that the GLSL of a Vertex function maps clip-space z from 0 to w onto
// OpenGL's -w to w before each return.
func TestCompileShaderUserVertexClipZ(t *testing.T) {
	const src = `//kage:unit pixels

package main

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4) (vec4, vec2, vec4, vec4) {
	if custom.x > 0 {
		return vec4(0), srcPos, color, custom
	}
	return imageDstProjection() * vec4(dstPos, custom.z, 1), srcPos, color, custom
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	return color
}
`
	const remap = "gl_Position.z = 2.0*gl_Position.z - gl_Position.w;\n"
	ir, err := graphics.CompileShader([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []glsl.GLSLVersion{glsl.GLSLVersionDefault, glsl.GLSLVersionES300} {
		vs, _ := glsl.Compile(ir, version)
		if got := strings.Count(vs, remap+"\t\treturn;\n") + strings.Count(vs, remap+"\treturn;\n"); got != 2 || strings.Count(vs, "return;") != 2 {
			t.Errorf("version %d: the remap must come before each of the 2 returns, but comes before %d:\n%s", version, got, vs)
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

const instancedVertexShaderSource = `//kage:unit pixels

package main

func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4, iDstPos vec2, iSrcPos vec2, iColor vec4, iCustom vec4) (vec4, vec2, vec4, vec4) {
	return imageDstProjection() * vec4(dstPos+iDstPos+iSrcPos, 0, 1), srcPos, color * iColor, custom + iCustom
}

func Fragment(dstPos vec4, srcPos vec2, color vec4) vec4 {
	return color
}
`

func TestCompileShaderInstancedVertex(t *testing.T) {
	ir, err := graphics.CompileShader([]byte(instancedVertexShaderSource))
	if err != nil {
		t.Fatal(err)
	}
	if !ir.UserVertex {
		t.Errorf("ir.UserVertex: got: false, want: true")
	}
	if got, want := len(ir.Attributes), 8; got != want {
		t.Errorf("len(ir.Attributes): got: %d, want: %d", got, want)
	}

	vs, _, _, _ := hlsl.Compile(ir)
	if want := "Varyings VSMain(float2 A0 : POSITION, float2 A1 : TEXCOORD, float4 A2 : COLOR0, float4 A3 : COLOR1, float2 A4 : TEXCOORD1, float2 A5 : TEXCOORD2, float4 A6 : COLOR4, float4 A7 : COLOR5) {"; !strings.Contains(vs, want) {
		t.Errorf("HLSL: the vertex shader must contain %q but does not:\n%s", want, vs)
	}

	m := msl.Compile(ir)
	for _, want := range []string{
		"struct Attributes {\n\tfloat2 M0;\n\tfloat2 M1;\n\tfloat4 M2;\n\tfloat4 M3;\n};",
		"struct InstanceAttributes {\n\tfloat2 M0;\n\tfloat2 M1;\n\tfloat4 M2;\n\tfloat4 M3;\n};",
		"\tuint iid [[instance_id]],\n\tconst device InstanceAttributes* instances [[buffer(2)]]",
		"instances[iid].M0",
		"instances[iid].M3",
	} {
		if !strings.Contains(m, want) {
			t.Errorf("MSL: the shader must contain %q but does not:\n%s", want, m)
		}
	}
}

func TestCompileShaderInstancedVertexSignature(t *testing.T) {
	cases := []struct {
		name   string
		vertex string
	}{
		{
			name:   "five attributes",
			vertex: "func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4, iDstPos vec2) (vec4, vec2, vec4, vec4) {\n\treturn vec4(dstPos+iDstPos, 0, 1), srcPos, color, custom\n}",
		},
		{
			name:   "wrong instance attribute type",
			vertex: "func Vertex(dstPos vec2, srcPos vec2, color vec4, custom vec4, iDstPos vec4, iSrcPos vec2, iColor vec4, iCustom vec4) (vec4, vec2, vec4, vec4) {\n\treturn iDstPos, srcPos, color, custom\n}",
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
	if vs, _ := glsl.Compile(ir, glsl.GLSLVersionDefault); strings.Contains(vs, "gl_Position.z") {
		t.Errorf("the builtin vertex shader must not remap z:\n%s", vs)
	}
	// The complete source, and then the program, must be the same as in v2.10.4.
	if got, want := graphics.CalcSourceID([]byte(src)).String(), "joozshohbaxb53d5o5joudhkre"; got != want {
		t.Errorf("graphics.CalcSourceID: got: %s, want: %s", got, want)
	}
}
