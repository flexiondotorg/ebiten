// Copyright 2022 The Ebitengine Authors
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

package gl

const (
	ARRAY_BUFFER          = 0x8892
	BLEND                 = 0x0BE2
	CLAMP_TO_EDGE         = 0x812F
	COLOR_ATTACHMENT0     = 0x8CE0
	COMPLETION_STATUS_KHR = 0x91B1
	DEPTH_ATTACHMENT      = 0x8D00
	DEPTH_BUFFER_BIT      = 0x0100
	DEPTH_COMPONENT       = 0x1902
	DEPTH_COMPONENT24     = 0x81A6
	DEPTH_TEST            = 0x0B71
	DRAW_FRAMEBUFFER      = 0x8CA9
	DST_ALPHA             = 0x0304
	DST_COLOR             = 0x0306
	DYNAMIC_DRAW          = 0x88E8
	ELEMENT_ARRAY_BUFFER  = 0x8893
	FALSE                 = 0
	FLOAT                 = 0x1406
	FRAGMENT_SHADER       = 0x8B30
	FRAMEBUFFER           = 0x8D40
	FRAMEBUFFER_BINDING   = 0x8CA6
	FRAMEBUFFER_COMPLETE  = 0x8CD5
	FUNC_ADD              = 0x8006
	FUNC_REVERSE_SUBTRACT = 0x800b
	FUNC_SUBTRACT         = 0x800a
	INFO_LOG_LENGTH       = 0x8B84
	LEQUAL                = 0x0203
	LINK_STATUS           = 0x8B82
	MAX                   = 0x8008
	MAX_TEXTURE_SIZE      = 0x0D33
	MIN                   = 0x8007
	NEAREST               = 0x2600
	NO_ERROR              = 0
	ONE                   = 1
	ONE_MINUS_DST_ALPHA   = 0x0305
	ONE_MINUS_DST_COLOR   = 0x0307
	ONE_MINUS_SRC_ALPHA   = 0x0303
	ONE_MINUS_SRC_COLOR   = 0x0301
	READ_FRAMEBUFFER      = 0x8CA8
	RGBA                  = 0x1908
	SCISSOR_TEST          = 0x0C11
	SRC_ALPHA             = 0x0302
	SRC_ALPHA_SATURATE    = 0x0308
	SRC_COLOR             = 0x0300
	STATIC_DRAW           = 0x88E4
	STREAM_DRAW           = 0x88E0
	TEXTURE0              = 0x84C0
	TEXTURE_2D            = 0x0DE1
	TEXTURE_MAG_FILTER    = 0x2800
	TEXTURE_MIN_FILTER    = 0x2801
	TEXTURE_WRAP_S        = 0x2802
	TEXTURE_WRAP_T        = 0x2803
	TRIANGLES             = 0x0004
	TRUE                  = 1
	UNPACK_ALIGNMENT      = 0x0CF5
	UNSIGNED_BYTE         = 0x1401
	UNSIGNED_INT          = 0x1405
	VERTEX_SHADER         = 0x8B31
	ZERO                  = 0
)

// Timer queries, from OpenGL 3.3, GL_ARB_timer_query, and GL_EXT_disjoint_timer_query.
const (
	EXTENSIONS             = 0x1F03
	GPU_DISJOINT_EXT       = 0x8FBB
	MAJOR_VERSION          = 0x821B
	MINOR_VERSION          = 0x821C
	NUM_EXTENSIONS         = 0x821D
	QUERY_RESULT           = 0x8866
	QUERY_RESULT_AVAILABLE = 0x8867
	TIME_ELAPSED           = 0x88BF
)

const (
	RENDERER = 0x1F01
	VENDOR   = 0x1F00
	VERSION  = 0x1F02
)
