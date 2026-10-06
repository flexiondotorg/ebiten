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
	"slices"
	"testing"
)

// testShown and testReleased stand for the display: the drawables that are shown, and the drawables released.
var testShown, testReleased []int

func testIsShown(x int) bool { return slices.Contains(testShown, x) }
func testRelease(x int)      { testReleased = append(testReleased, x) }

func TestPresentQueue(t *testing.T) {
	testShown, testReleased = nil, nil
	var q presentQueue[int]
	for i := 1; i <= 3; i++ {
		q.push(i, testRelease)
	}
	if n := q.update(testIsShown, testRelease); n != 3 || len(testReleased) != 0 {
		t.Fatalf("nothing shown: %d queued, released %v, want 3 and none", n, testReleased)
	}
	// Drawable 2 is shown, so drawable 1 is shown or dropped too.
	testShown = []int{2}
	if n := q.update(testIsShown, testRelease); n != 1 || !slices.Equal(testReleased, []int{1, 2}) {
		t.Fatalf("2 shown: %d queued, released %v, want 1 and [1 2]", n, testReleased)
	}
	q.push(4, testRelease)
	testShown = append(testShown, 4)
	if n := q.update(testIsShown, testRelease); n != 0 || !slices.Equal(testReleased, []int{1, 2, 3, 4}) {
		t.Fatalf("4 shown: %d queued, released %v, want 0 and [1 2 3 4]", n, testReleased)
	}
}

func TestPresentQueueFull(t *testing.T) {
	testShown, testReleased = nil, nil
	var q presentQueue[int]
	for i := 1; i <= presentQueueSize+2; i++ {
		q.push(i, testRelease)
	}
	if q.n != presentQueueSize || !slices.Equal(testReleased, []int{1, 2}) || q.items[0] != 3 {
		t.Errorf("full queue: %d queued from %d, released %v, want %d from 3 and [1 2]", q.n, q.items[0], testReleased, presentQueueSize)
	}
}

func TestPresentQueueAllocs(t *testing.T) {
	var q presentQueue[int]
	shown := func(x int) bool { return x%3 == 0 }
	release := func(int) {}
	i := 0
	if n := testing.AllocsPerRun(100, func() {
		i++
		q.push(i, release)
		q.update(shown, release)
	}); n != 0 {
		t.Errorf("%v allocations a present, want 0", n)
	}
}
