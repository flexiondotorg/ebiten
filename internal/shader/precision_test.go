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

package shader_test

import (
	"strings"
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/graphics"
	"github.com/hajimehoshi/ebiten/v2/internal/shader"
	"github.com/hajimehoshi/ebiten/v2/internal/shaderir/glsl"
)

func TestCompilePrecisionDirective(t *testing.T) {
	src := `//kage:unit pixels

package main

// luma has medium precision.
//
//kage:precision mediump
func luma(c vec3, n int) float {
	w := vec3(0.299, 0.587, 0.114)
	var a [2]vec2
	return dot(c, w)*float(n) + a[1].x
}

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	p := texCoord * 2
	return vec4(luma(color.rgb, 1), p, 1)
}`
	s, err := graphics.CompileShader([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	_, fs := glsl.Compile(s, glsl.GLSLVersionES300)
	for _, want := range []string{
		"mediump float F0(in mediump vec3 l0, in int l1)",
		"mediump vec3 l2 = vec3(0);",
		"mediump vec2 l3[2];",
	} {
		if !strings.Contains(fs, want) {
			t.Errorf("the fragment shader must contain %q, but got:\n%s", want, fs)
		}
	}
	// Only the function of the directive has medium precision: two in its prototype, two in its
	// definition, and its two float locals.
	if got, want := strings.Count(fs, "mediump "), 6; got != want {
		t.Errorf("the count of mediump qualifiers: got: %d, want: %d\n%s", got, want, fs)
	}
}

func TestCompilePrecisionDirectiveErrors(t *testing.T) {
	srcs := []string{
		`package main

//kage:precision lowp
func luma(c vec3) float {
	return c.r
}

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	return vec4(luma(color.rgb))
}`,
		`package main

//kage:precision mediump
func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	return color
}`,
		`package main

//kage:precision mediump
//kage:precision mediump
func luma(c vec3) float {
	return c.r
}

func Fragment(position vec4, texCoord vec2, color vec4) vec4 {
	return vec4(luma(color.rgb))
}`,
	}
	for _, src := range srcs {
		if _, err := shader.Compile([]byte(src), "Vertex", "Fragment", 0); err == nil {
			t.Errorf("Compile must return an error for an invalid //kage:precision directive, but got nil:\n%s", src)
		}
	}
}
