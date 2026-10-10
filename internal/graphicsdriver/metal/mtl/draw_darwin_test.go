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

package mtl_test

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2/internal/graphicsdriver/metal/mtl"
)

// TestGPUTimes checks the GPU start and end times of a completed command buffer.
func TestGPUTimes(t *testing.T) {
	device, err := mtl.CreateSystemDefaultDevice()
	if err != nil {
		t.Skip(err)
	}
	cq, err := device.NewCommandQueue()
	if err != nil {
		t.Fatal(err)
	}
	cb, err := cq.CommandBuffer()
	if err != nil {
		t.Fatal(err)
	}
	cb.Retain()
	defer cb.Release()
	cb.Commit()
	cb.WaitUntilCompleted()

	start, end := cb.GPUStartTime(), cb.GPUEndTime()
	if start <= 0 || end < start {
		t.Errorf("GPU times: got %v to %v, want a positive start and an end at or after it", start, end)
	}
}
