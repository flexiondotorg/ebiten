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

package metal

import (
	"fmt"
	"sync"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	machAbsoluteTime uintptr
	hostTick         float64 // Seconds of one tick of mach_absolute_time.
	hostTimeOnce     sync.Once
)

// hostTime returns the host time in seconds, on the clock of the GPU start and end times of a
// command buffer.
func hostTime() float64 {
	hostTimeOnce.Do(func() {
		lib, err := purego.Dlopen("/usr/lib/libSystem.B.dylib", purego.RTLD_GLOBAL|purego.RTLD_NOW)
		if err != nil {
			panic(fmt.Sprintf("metal: dlopen libSystem failed: %v", err))
		}
		if machAbsoluteTime, err = purego.Dlsym(lib, "mach_absolute_time"); err != nil {
			panic(fmt.Sprintf("metal: dlsym mach_absolute_time failed: %v", err))
		}
		timebaseInfo, err := purego.Dlsym(lib, "mach_timebase_info")
		if err != nil {
			panic(fmt.Sprintf("metal: dlsym mach_timebase_info failed: %v", err))
		}
		var info struct{ numer, denom uint32 }
		purego.SyscallN(timebaseInfo, uintptr(unsafe.Pointer(&info)))
		hostTick = float64(info.numer) / float64(max(info.denom, 1)) / 1e9
	})
	t, _, _ := purego.SyscallN(machAbsoluteTime)
	return float64(t) * hostTick
}
