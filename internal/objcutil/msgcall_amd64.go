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

// intArgRegisters is the number of integer argument registers in the System V ABI for amd64.
const intArgRegisters = 6

// packStruct appends a struct argument of more than 16 bytes after the n integer arguments in
// c.args, and returns the new number of arguments.
//
// The System V ABI for amd64, which Intel Macs use, passes such a struct in memory: its words go
// on the stack in order, after the integer arguments that do not fit in the 6 registers.
// purego.SyscallN puts the arguments from the 7th onwards on the stack, so the unused registers
// before the struct are filled with zeros.
//
// This form is not tested on an Intel Mac.
func (c *msgCall) packStruct(n int, words []uintptr) int {
	start := max(n, intArgRegisters)
	if start+len(words) > len(c.args) {
		panic("objcutil: too many arguments")
	}
	clear(c.args[n:start])
	return start + copy(c.args[start:], words)
}
