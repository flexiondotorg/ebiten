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

// presentQueueSize is the most drawables that a presentQueue holds. No more than maximumDrawableCount
// drawables can wait for presentation, so the queue is full only when the tracking is stuck.
const presentQueueSize = 8

// presentQueue holds the drawables that are queued for presentation and not known to be shown, oldest
// first. The drawables of a layer are presented in order, so when a drawable is shown, every older
// drawable is shown or dropped.
type presentQueue[T any] struct {
	items [presentQueueSize]T
	n     int
}

// push adds x as the newest drawable. When the queue is full, it releases and forgets the oldest.
func (q *presentQueue[T]) push(x T, release func(T)) {
	if q.n == len(q.items) {
		release(q.items[0])
		copy(q.items[:], q.items[1:])
		q.n--
	}
	q.items[q.n] = x
	q.n++
}

// update releases the newest shown drawable and every older drawable, and returns the number of the
// drawables left.
func (q *presentQueue[T]) update(shown func(T) bool, release func(T)) int {
	for i := q.n - 1; i >= 0; i-- {
		if !shown(q.items[i]) {
			continue
		}
		for _, x := range q.items[:i+1] {
			release(x)
		}
		k := copy(q.items[:], q.items[i+1:q.n])
		clear(q.items[k:q.n])
		q.n = k
		break
	}
	return q.n
}
