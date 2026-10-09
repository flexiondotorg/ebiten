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

//go:build (darwin || freebsd || linux || netbsd || windows) && !nintendosdk && !playstation5

package gl

import (
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego"
)

type defaultContext struct {
	// The query functions are optional: hasTimerQuery reports whether the context has them.
	gpBeginQuery        uintptr
	gpEndQuery          uintptr
	gpGenQueries        uintptr
	gpGetQueryObjectuiv uintptr
	gpGetStringi        uintptr
	hasTimerQuery       bool
	gpGetString         uintptr

	gpActiveTexture           uintptr
	gpAttachShader            uintptr
	gpBindAttribLocation      uintptr
	gpBindBuffer              uintptr
	gpBindFramebuffer         uintptr
	gpBindTexture             uintptr
	gpBindVertexArray         uintptr
	gpBlendEquationSeparate   uintptr
	gpBlendFuncSeparate       uintptr
	gpBlitFramebuffer         uintptr
	gpBufferData              uintptr
	gpBufferSubData           uintptr
	gpCheckFramebufferStatus  uintptr
	gpClear                   uintptr
	gpCompileShader           uintptr
	gpCreateProgram           uintptr
	gpCreateShader            uintptr
	gpDeleteBuffers           uintptr
	gpDeleteFramebuffers      uintptr
	gpDeleteProgram           uintptr
	gpDeleteShader            uintptr
	gpDeleteTextures          uintptr
	gpDeleteVertexArrays      uintptr
	gpDepthFunc               uintptr
	gpDepthMask               uintptr
	gpDisable                 uintptr
	gpDrawElements            uintptr
	gpDrawElementsInstanced   uintptr
	gpEnable                  uintptr
	gpEnableVertexAttribArray uintptr
	gpFinish                  uintptr
	gpFlush                   uintptr
	gpFramebufferTexture2D    uintptr
	gpGenBuffers              uintptr
	gpGenFramebuffers         uintptr
	gpGenTextures             uintptr
	gpGenVertexArrays         uintptr
	gpGetError                uintptr
	gpGetIntegerv             uintptr
	gpGetProgramInfoLog       uintptr
	gpGetProgramiv            uintptr
	gpGetShaderInfoLog        uintptr
	gpGetShaderiv             uintptr
	gpGetUniformLocation      uintptr
	gpIsProgram               uintptr
	gpLinkProgram             uintptr
	gpPixelStorei             uintptr
	gpReadPixels              uintptr
	gpScissor                 uintptr
	gpShaderSource            uintptr
	gpTexImage2D              uintptr
	gpTexParameteri           uintptr
	gpTexSubImage2D           uintptr
	gpUniform1fv              uintptr
	gpUniform1i               uintptr
	gpUniform1iv              uintptr
	gpUniform2fv              uintptr
	gpUniform2iv              uintptr
	gpUniform3fv              uintptr
	gpUniform3iv              uintptr
	gpUniform4fv              uintptr
	gpUniform4iv              uintptr
	gpUniformMatrix2fv        uintptr
	gpUniformMatrix3fv        uintptr
	gpUniformMatrix4fv        uintptr
	gpUseProgram              uintptr
	gpVertexAttribDivisor     uintptr
	gpVertexAttribPointer     uintptr
	gpViewport                uintptr

	// hasInstancing reports whether the optional functions of the instanced draws are loaded.
	hasInstancing bool

	// gpInvalidateFramebuffer is optional, and depthAttachment holds its attachment argument.
	gpInvalidateFramebuffer uintptr
	depthAttachment         uint32

	// args holds the arguments of call.
	args [15]uintptr

	// pinner pins the Go objects of the calls that pass data in each frame.
	pinner runtime.Pinner
	out    uint32

	isES bool
}

func NewDefaultContext() (Context, error) {
	ctx := &defaultContext{}
	if err := ctx.init(); err != nil {
		return nil, err
	}
	return ctx, nil
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// call calls purego.SyscallN with arguments that hold no Go pointers. purego.SyscallN allocates
// its variadic arguments on the heap, so call passes them in c.args instead. Pass a Go pointer
// to purego.SyscallN directly, or pin its object with c.pinner around call, so that the object
// stays alive and in place.
func (c *defaultContext) call(fn uintptr, args ...uintptr) (r1, r2, err uintptr) {
	n := copy(c.args[:], args)
	return purego.SyscallN(fn, c.args[:n]...)
}

func (c *defaultContext) IsES() bool {
	return c.isES
}

func (c *defaultContext) ActiveTexture(texture uint32) {
	c.call(c.gpActiveTexture, uintptr(texture))
}

func (c *defaultContext) AttachShader(program uint32, shader uint32) {
	c.call(c.gpAttachShader, uintptr(program), uintptr(shader))
}

func (c *defaultContext) BindAttribLocation(program uint32, index uint32, name string) {
	cname, free := cStr(name)
	defer free()
	purego.SyscallN(c.gpBindAttribLocation, uintptr(program), uintptr(index), uintptr(unsafe.Pointer(cname)))
}

func (c *defaultContext) BindBuffer(target uint32, buffer uint32) {
	c.call(c.gpBindBuffer, uintptr(target), uintptr(buffer))
}

func (c *defaultContext) BindFramebuffer(target uint32, framebuffer uint32) {
	c.call(c.gpBindFramebuffer, uintptr(target), uintptr(framebuffer))
}

func (c *defaultContext) BindTexture(target uint32, texture uint32) {
	c.call(c.gpBindTexture, uintptr(target), uintptr(texture))
}

func (c *defaultContext) BindVertexArray(array uint32) {
	c.call(c.gpBindVertexArray, uintptr(array))
}

func (c *defaultContext) BlendEquationSeparate(modeRGB uint32, modeAlpha uint32) {
	c.call(c.gpBlendEquationSeparate, uintptr(modeRGB), uintptr(modeAlpha))
}

func (c *defaultContext) BlendFuncSeparate(srcRGB uint32, dstRGB uint32, srcAlpha uint32, dstAlpha uint32) {
	c.call(c.gpBlendFuncSeparate, uintptr(srcRGB), uintptr(dstRGB), uintptr(srcAlpha), uintptr(dstAlpha))
}

func (c *defaultContext) BlitFramebuffer(srcX0, srcY0, srcX1, srcY1, dstX0, dstY0, dstX1, dstY1 int32, mask uint32, filter uint32) {
	c.call(c.gpBlitFramebuffer, uintptr(srcX0), uintptr(srcY0), uintptr(srcX1), uintptr(srcY1), uintptr(dstX0), uintptr(dstY0), uintptr(dstX1), uintptr(dstY1), uintptr(mask), uintptr(filter))
}

func (c *defaultContext) BufferInit(target uint32, size int, usage uint32) {
	c.call(c.gpBufferData, uintptr(target), uintptr(size), 0, uintptr(usage))
}

func (c *defaultContext) BufferSubData(target uint32, offset int, data []byte) {
	c.pinner.Pin(&data[0])
	c.call(c.gpBufferSubData, uintptr(target), uintptr(offset), uintptr(len(data)), uintptr(unsafe.Pointer(&data[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) CheckFramebufferStatus(target uint32) uint32 {
	ret, _, _ := c.call(c.gpCheckFramebufferStatus, uintptr(target))
	return uint32(ret)
}

func (c *defaultContext) Clear(mask uint32) {
	c.call(c.gpClear, uintptr(mask))
}

func (c *defaultContext) CompileShader(shader uint32) {
	c.call(c.gpCompileShader, uintptr(shader))
}

func (c *defaultContext) CreateBuffer() uint32 {
	var buffer uint32
	purego.SyscallN(c.gpGenBuffers, 1, uintptr(unsafe.Pointer(&buffer)))
	return buffer
}

func (c *defaultContext) CreateFramebuffer() uint32 {
	var framebuffer uint32
	purego.SyscallN(c.gpGenFramebuffers, 1, uintptr(unsafe.Pointer(&framebuffer)))
	return framebuffer
}

func (c *defaultContext) CreateProgram() uint32 {
	ret, _, _ := c.call(c.gpCreateProgram)
	return uint32(ret)
}

func (c *defaultContext) CreateShader(xtype uint32) uint32 {
	ret, _, _ := c.call(c.gpCreateShader, uintptr(xtype))
	return uint32(ret)
}

func (c *defaultContext) CreateTexture() uint32 {
	var texture uint32
	purego.SyscallN(c.gpGenTextures, 1, uintptr(unsafe.Pointer(&texture)))
	return texture
}

func (c *defaultContext) CreateVertexArray() uint32 {
	var array uint32
	purego.SyscallN(c.gpGenVertexArrays, 1, uintptr(unsafe.Pointer(&array)))
	return array
}

func (c *defaultContext) DeleteBuffer(buffer uint32) {
	purego.SyscallN(c.gpDeleteBuffers, 1, uintptr(unsafe.Pointer(&buffer)))
}

func (c *defaultContext) DeleteFramebuffer(framebuffer uint32) {
	purego.SyscallN(c.gpDeleteFramebuffers, 1, uintptr(unsafe.Pointer(&framebuffer)))
}

func (c *defaultContext) DeleteProgram(program uint32) {
	// A program is no longer valid after a context loss and must not be deleted.
	if !c.IsProgram(program) {
		return
	}
	c.call(c.gpDeleteProgram, uintptr(program))
}

func (c *defaultContext) DeleteShader(shader uint32) {
	c.call(c.gpDeleteShader, uintptr(shader))
}

func (c *defaultContext) DeleteTexture(texture uint32) {
	purego.SyscallN(c.gpDeleteTextures, 1, uintptr(unsafe.Pointer(&texture)))
}

func (c *defaultContext) DeleteVertexArray(array uint32) {
	purego.SyscallN(c.gpDeleteVertexArrays, 1, uintptr(unsafe.Pointer(&array)))
}

func (c *defaultContext) DepthFunc(xfunc uint32) {
	c.call(c.gpDepthFunc, uintptr(xfunc))
}

func (c *defaultContext) DepthMask(flag bool) {
	c.call(c.gpDepthMask, uintptr(boolToInt(flag)))
}

func (c *defaultContext) Disable(cap uint32) {
	c.call(c.gpDisable, uintptr(cap))
}

func (c *defaultContext) DrawElements(mode uint32, count int32, xtype uint32, offset int) {
	c.call(c.gpDrawElements, uintptr(mode), uintptr(count), uintptr(xtype), uintptr(offset))
}

func (c *defaultContext) DrawElementsInstanced(mode uint32, count int32, xtype uint32, offset int, instanceCount int32) {
	c.call(c.gpDrawElementsInstanced, uintptr(mode), uintptr(count), uintptr(xtype), uintptr(offset), uintptr(instanceCount))
}

func (c *defaultContext) Enable(cap uint32) {
	c.call(c.gpEnable, uintptr(cap))
}

func (c *defaultContext) EnableVertexAttribArray(index uint32) {
	c.call(c.gpEnableVertexAttribArray, uintptr(index))
}

func (c *defaultContext) Finish() {
	c.call(c.gpFinish)
}

func (c *defaultContext) Flush() {
	c.call(c.gpFlush)
}

func (c *defaultContext) FramebufferTexture2D(target uint32, attachment uint32, textarget uint32, texture uint32, level int32) {
	c.call(c.gpFramebufferTexture2D, uintptr(target), uintptr(attachment), uintptr(textarget), uintptr(texture), uintptr(level))
}

func (c *defaultContext) GetError() uint32 {
	ret, _, _ := c.call(c.gpGetError)
	return uint32(ret)
}

func (c *defaultContext) GetExtension(name string) any {
	return nil
}

func (c *defaultContext) GetInteger(pname uint32) int {
	c.pinner.Pin(&c.out)
	c.call(c.gpGetIntegerv, uintptr(pname), uintptr(unsafe.Pointer(&c.out)))
	c.pinner.Unpin()
	return int(int32(c.out))
}

func (c *defaultContext) GetProgramInfoLog(program uint32) string {
	bufSize := c.GetProgrami(program, INFO_LOG_LENGTH)
	if bufSize == 0 {
		return ""
	}
	infoLog := make([]byte, bufSize)
	purego.SyscallN(c.gpGetProgramInfoLog, uintptr(program), uintptr(bufSize), 0, uintptr(unsafe.Pointer(&infoLog[0])))
	return string(infoLog)
}

func (c *defaultContext) GetProgrami(program uint32, pname uint32) int {
	var dst int32
	purego.SyscallN(c.gpGetProgramiv, uintptr(program), uintptr(pname), uintptr(unsafe.Pointer(&dst)))
	return int(dst)
}

func (c *defaultContext) GetShaderInfoLog(shader uint32) string {
	bufSize := c.GetShaderi(shader, INFO_LOG_LENGTH)
	if bufSize == 0 {
		return ""
	}
	infoLog := make([]byte, bufSize)
	purego.SyscallN(c.gpGetShaderInfoLog, uintptr(shader), uintptr(bufSize), 0, uintptr(unsafe.Pointer(&infoLog[0])))
	return string(infoLog)
}

func (c *defaultContext) GetShaderi(shader uint32, pname uint32) int {
	var dst int32
	purego.SyscallN(c.gpGetShaderiv, uintptr(shader), uintptr(pname), uintptr(unsafe.Pointer(&dst)))
	return int(dst)
}

func (c *defaultContext) GetUniformLocation(program uint32, name string) int32 {
	cname, free := cStr(name)
	defer free()
	ret, _, _ := purego.SyscallN(c.gpGetUniformLocation, uintptr(program), uintptr(unsafe.Pointer(cname)))
	return int32(ret)
}

func (c *defaultContext) IsProgram(program uint32) bool {
	ret, _, _ := c.call(c.gpIsProgram, uintptr(program))
	return byte(ret) != 0
}

func (c *defaultContext) LinkProgram(program uint32) {
	c.call(c.gpLinkProgram, uintptr(program))
}

func (c *defaultContext) PixelStorei(pname uint32, param int32) {
	c.call(c.gpPixelStorei, uintptr(pname), uintptr(param))
}

func (c *defaultContext) ReadPixels(dst []byte, x int32, y int32, width int32, height int32, format uint32, xtype uint32) {
	purego.SyscallN(c.gpReadPixels, uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(format), uintptr(xtype), uintptr(unsafe.Pointer(&dst[0])))
	runtime.KeepAlive(dst)
}

func (c *defaultContext) Scissor(x int32, y int32, width int32, height int32) {
	c.call(c.gpScissor, uintptr(x), uintptr(y), uintptr(width), uintptr(height))
}

func (c *defaultContext) ShaderSource(shader uint32, xstring string) {
	cstring, free := cStr(xstring)
	defer free()
	purego.SyscallN(c.gpShaderSource, uintptr(shader), 1, uintptr(unsafe.Pointer(&cstring)), 0)
}

func (c *defaultContext) TexImage2D(target uint32, level int32, internalformat int32, width int32, height int32, format uint32, xtype uint32, pixels []byte) {
	var ptr *byte
	if len(pixels) > 0 {
		ptr = &pixels[0]
	}
	purego.SyscallN(c.gpTexImage2D, uintptr(target), uintptr(level), uintptr(internalformat), uintptr(width), uintptr(height), 0, uintptr(format), uintptr(xtype), uintptr(unsafe.Pointer(ptr)))
	runtime.KeepAlive(pixels)
}

func (c *defaultContext) TexParameteri(target uint32, pname uint32, param int32) {
	c.call(c.gpTexParameteri, uintptr(target), uintptr(pname), uintptr(param))
}

func (c *defaultContext) TexSubImage2D(target uint32, level int32, xoffset int32, yoffset int32, width int32, height int32, format uint32, xtype uint32, pixels []byte) {
	purego.SyscallN(c.gpTexSubImage2D, uintptr(target), uintptr(level), uintptr(xoffset), uintptr(yoffset), uintptr(width), uintptr(height), uintptr(format), uintptr(xtype), uintptr(unsafe.Pointer(&pixels[0])))
	runtime.KeepAlive(pixels)
}

func (c *defaultContext) Uniform1fv(location int32, value []float32) {
	c.pinner.Pin(&value[0])
	c.call(c.gpUniform1fv, uintptr(location), uintptr(len(value)), uintptr(unsafe.Pointer(&value[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) Uniform1i(location int32, v0 int32) {
	c.call(c.gpUniform1i, uintptr(location), uintptr(v0))
}

func (c *defaultContext) Uniform1iv(location int32, value []int32) {
	c.pinner.Pin(&value[0])
	c.call(c.gpUniform1iv, uintptr(location), uintptr(len(value)), uintptr(unsafe.Pointer(&value[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) Uniform2fv(location int32, value []float32) {
	c.pinner.Pin(&value[0])
	c.call(c.gpUniform2fv, uintptr(location), uintptr(len(value)/2), uintptr(unsafe.Pointer(&value[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) Uniform2iv(location int32, value []int32) {
	c.pinner.Pin(&value[0])
	c.call(c.gpUniform2iv, uintptr(location), uintptr(len(value)/2), uintptr(unsafe.Pointer(&value[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) Uniform3fv(location int32, value []float32) {
	c.pinner.Pin(&value[0])
	c.call(c.gpUniform3fv, uintptr(location), uintptr(len(value)/3), uintptr(unsafe.Pointer(&value[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) Uniform3iv(location int32, value []int32) {
	c.pinner.Pin(&value[0])
	c.call(c.gpUniform3iv, uintptr(location), uintptr(len(value)/3), uintptr(unsafe.Pointer(&value[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) Uniform4fv(location int32, value []float32) {
	c.pinner.Pin(&value[0])
	c.call(c.gpUniform4fv, uintptr(location), uintptr(len(value)/4), uintptr(unsafe.Pointer(&value[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) Uniform4iv(location int32, value []int32) {
	c.pinner.Pin(&value[0])
	c.call(c.gpUniform4iv, uintptr(location), uintptr(len(value)/4), uintptr(unsafe.Pointer(&value[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) UniformMatrix2fv(location int32, value []float32) {
	c.pinner.Pin(&value[0])
	c.call(c.gpUniformMatrix2fv, uintptr(location), uintptr(len(value)/4), 0, uintptr(unsafe.Pointer(&value[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) UniformMatrix3fv(location int32, value []float32) {
	c.pinner.Pin(&value[0])
	c.call(c.gpUniformMatrix3fv, uintptr(location), uintptr(len(value)/9), 0, uintptr(unsafe.Pointer(&value[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) UniformMatrix4fv(location int32, value []float32) {
	c.pinner.Pin(&value[0])
	c.call(c.gpUniformMatrix4fv, uintptr(location), uintptr(len(value)/16), 0, uintptr(unsafe.Pointer(&value[0])))
	c.pinner.Unpin()
}

func (c *defaultContext) UseProgram(program uint32) {
	c.call(c.gpUseProgram, uintptr(program))
}

func (c *defaultContext) VertexAttribDivisor(index uint32, divisor uint32) {
	c.call(c.gpVertexAttribDivisor, uintptr(index), uintptr(divisor))
}

func (c *defaultContext) VertexAttribPointer(index uint32, size int32, xtype uint32, normalized bool, stride int32, offset int) {
	c.call(c.gpVertexAttribPointer, uintptr(index), uintptr(size), uintptr(xtype), uintptr(boolToInt(normalized)), uintptr(stride), uintptr(offset))
}

func (c *defaultContext) Viewport(x int32, y int32, width int32, height int32) {
	c.call(c.gpViewport, uintptr(x), uintptr(y), uintptr(width), uintptr(height))
}

func (c *defaultContext) LoadFunctions() error {
	g := procAddressGetter{ctx: c}

	c.gpActiveTexture = g.get("glActiveTexture")
	c.gpAttachShader = g.get("glAttachShader")
	c.gpBindAttribLocation = g.get("glBindAttribLocation")
	c.gpBindBuffer = g.get("glBindBuffer")
	c.gpBindFramebuffer = g.get("glBindFramebuffer")
	c.gpBindTexture = g.get("glBindTexture")
	c.gpBindVertexArray = g.get("glBindVertexArray")
	c.gpBlendEquationSeparate = g.get("glBlendEquationSeparate")
	c.gpBlendFuncSeparate = g.get("glBlendFuncSeparate")
	c.gpBlitFramebuffer = g.get("glBlitFramebuffer")
	c.gpBufferData = g.get("glBufferData")
	c.gpBufferSubData = g.get("glBufferSubData")
	c.gpCheckFramebufferStatus = g.get("glCheckFramebufferStatus")
	c.gpClear = g.get("glClear")
	c.gpCompileShader = g.get("glCompileShader")
	c.gpCreateProgram = g.get("glCreateProgram")
	c.gpCreateShader = g.get("glCreateShader")
	c.gpDeleteBuffers = g.get("glDeleteBuffers")
	c.gpDeleteFramebuffers = g.get("glDeleteFramebuffers")
	c.gpDeleteProgram = g.get("glDeleteProgram")
	c.gpDeleteShader = g.get("glDeleteShader")
	c.gpDeleteTextures = g.get("glDeleteTextures")
	c.gpDeleteVertexArrays = g.get("glDeleteVertexArrays")
	c.gpDepthFunc = g.get("glDepthFunc")
	c.gpDepthMask = g.get("glDepthMask")
	c.gpDisable = g.get("glDisable")
	c.gpDrawElements = g.get("glDrawElements")
	c.gpEnable = g.get("glEnable")
	c.gpEnableVertexAttribArray = g.get("glEnableVertexAttribArray")
	c.gpFinish = g.get("glFinish")
	c.gpFlush = g.get("glFlush")
	c.gpFramebufferTexture2D = g.get("glFramebufferTexture2D")
	c.gpGenBuffers = g.get("glGenBuffers")
	c.gpGenFramebuffers = g.get("glGenFramebuffers")
	c.gpGenTextures = g.get("glGenTextures")
	c.gpGenVertexArrays = g.get("glGenVertexArrays")
	c.gpGetError = g.get("glGetError")
	c.gpGetIntegerv = g.get("glGetIntegerv")
	c.gpGetProgramInfoLog = g.get("glGetProgramInfoLog")
	c.gpGetProgramiv = g.get("glGetProgramiv")
	c.gpGetShaderInfoLog = g.get("glGetShaderInfoLog")
	c.gpGetShaderiv = g.get("glGetShaderiv")
	c.gpGetUniformLocation = g.get("glGetUniformLocation")
	c.gpIsProgram = g.get("glIsProgram")
	c.gpLinkProgram = g.get("glLinkProgram")
	c.gpPixelStorei = g.get("glPixelStorei")
	c.gpReadPixels = g.get("glReadPixels")
	c.gpScissor = g.get("glScissor")
	c.gpShaderSource = g.get("glShaderSource")
	c.gpTexImage2D = g.get("glTexImage2D")
	c.gpTexParameteri = g.get("glTexParameteri")
	c.gpTexSubImage2D = g.get("glTexSubImage2D")
	c.gpUniform1fv = g.get("glUniform1fv")
	c.gpUniform1i = g.get("glUniform1i")
	c.gpUniform1iv = g.get("glUniform1iv")
	c.gpUniform2fv = g.get("glUniform2fv")
	c.gpUniform2iv = g.get("glUniform2iv")
	c.gpUniform3fv = g.get("glUniform3fv")
	c.gpUniform3iv = g.get("glUniform3iv")
	c.gpUniform4fv = g.get("glUniform4fv")
	c.gpUniform4iv = g.get("glUniform4iv")
	c.gpUniformMatrix2fv = g.get("glUniformMatrix2fv")
	c.gpUniformMatrix3fv = g.get("glUniformMatrix3fv")
	c.gpUniformMatrix4fv = g.get("glUniformMatrix4fv")
	c.gpUseProgram = g.get("glUseProgram")
	c.gpVertexAttribPointer = g.get("glVertexAttribPointer")
	c.gpViewport = g.get("glViewport")

	if err := g.error(); err != nil {
		return err
	}

	// A missing query function must not fail the context.
	gq := procAddressGetter{ctx: c}
	c.gpBeginQuery = gq.get("glBeginQuery")
	c.gpEndQuery = gq.get("glEndQuery")
	c.gpGenQueries = gq.get("glGenQueries")
	c.gpGetQueryObjectuiv = gq.get("glGetQueryObjectuiv")
	c.gpGetStringi = gq.get("glGetStringi")
	c.hasTimerQuery = gq.error() == nil && c.timerQuerySupported()
	gs := procAddressGetter{ctx: c}
	c.gpGetString = gs.get("glGetString")

	// A missing function of the instanced draws must not fail the context.
	// glVertexAttribDivisor is core in OpenGL 3.3, and the context might be OpenGL 3.2.
	gi := procAddressGetter{ctx: c}
	c.gpDrawElementsInstanced = gi.get("glDrawElementsInstanced")
	c.gpVertexAttribDivisor = gi.get("glVertexAttribDivisor")
	c.hasInstancing = gi.error() == nil

	// glInvalidateFramebuffer is core in OpenGL ES 3.0 and OpenGL 4.3, and might be missing.
	gd := procAddressGetter{ctx: c}
	c.gpInvalidateFramebuffer = gd.get("glInvalidateFramebuffer")
	return nil
}

// InvalidateDepth invalidates the depth attachment of the bound framebuffer on OpenGL ES, where the call is core, and
// does nothing elsewhere. It is not part of Context.
func (c *defaultContext) InvalidateDepth() {
	if c.isES && c.gpInvalidateFramebuffer != 0 {
		c.depthAttachment = DEPTH_ATTACHMENT
		c.pinner.Pin(&c.depthAttachment)
		c.call(c.gpInvalidateFramebuffer, FRAMEBUFFER, 1, uintptr(unsafe.Pointer(&c.depthAttachment)))
		c.pinner.Unpin()
	}
}

func (c *defaultContext) HasInstancing() bool {
	return c.hasInstancing
}

// cStr takes a Go string (with or without null-termination)
// and returns the C counterpart.
//
// The bytes are Go-managed memory, so the returned function frees nothing. It
// must be called once the string is no longer used, as it keeps it alive until
// then.
func cStr(str string) (cstr *byte, free func()) {
	bs := []byte(str)
	if len(bs) == 0 || bs[len(bs)-1] != 0 {
		bs = append(bs, 0)
	}
	return &bs[0], func() {
		runtime.KeepAlive(bs)
		bs = nil
	}
}

func (c *defaultContext) HasTimerQuery() bool {
	return c.hasTimerQuery
}

func (c *defaultContext) BeginQuery(target uint32, query uint32) {
	c.call(c.gpBeginQuery, uintptr(target), uintptr(query))
}

func (c *defaultContext) CreateQuery() uint32 {
	var query uint32
	purego.SyscallN(c.gpGenQueries, 1, uintptr(unsafe.Pointer(&query)))
	return query
}

func (c *defaultContext) EndQuery(target uint32) {
	c.call(c.gpEndQuery, uintptr(target))
}

func (c *defaultContext) GetQueryObjectui(query uint32, pname uint32) uint32 {
	c.pinner.Pin(&c.out)
	c.call(c.gpGetQueryObjectuiv, uintptr(query), uintptr(pname), uintptr(unsafe.Pointer(&c.out)))
	c.pinner.Unpin()
	return c.out
}

// timerQuerySupported reports whether the context measures TIME_ELAPSED. The context must be current.
func (c *defaultContext) timerQuerySupported() bool {
	if !c.isES {
		if major, minor := c.GetInteger(MAJOR_VERSION), c.GetInteger(MINOR_VERSION); major > 3 || major == 3 && minor >= 3 {
			return true
		}
	}
	ext := "GL_ARB_timer_query"
	if c.isES {
		ext = "GL_EXT_disjoint_timer_query"
	}
	var getStringi func(name uint32, index uint32) string
	purego.RegisterFunc(&getStringi, c.gpGetStringi)
	for i := range c.GetInteger(NUM_EXTENSIONS) {
		if getStringi(EXTENSIONS, uint32(i)) == ext {
			return true
		}
	}
	return false
}

// GetString returns the string name of the context, for example RENDERER, or empty when the context
// has no glGetString. It is not part of Context: the renderer name reads it once, after the context exists.
func (c *defaultContext) GetString(name uint32) string {
	if c.gpGetString == 0 {
		return ""
	}
	var getString func(name uint32) string
	purego.RegisterFunc(&getString, c.gpGetString)
	return getString(name)
}
