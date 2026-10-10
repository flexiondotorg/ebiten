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

//go:build darwin || linux

package objcutil

import "unsafe"

// packStruct appends a struct argument of more than 16 bytes after the n integer arguments in
// c.args, and returns the new number of arguments.
//
// The procedure call standard for arm64, which Apple's platforms follow for this case, passes such
// a struct by reference: the caller copies it to memory and passes a pointer in place of it. The
// copy is in c, which stays pinned until the caller unpins it after the call.
func (c *msgCall) packStruct(n int, words []uintptr) int {
	if n+1 > len(c.args) {
		panic("objcutil: too many arguments")
	}
	copy(c.structArg[:], words)
	c.pinner.Pin(c)
	c.args[n] = uintptr(unsafe.Pointer(&c.structArg))
	return n + 1
}
