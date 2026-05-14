// Copyright 2021 ByteDance Inc.
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

package mbuffer

import (
	"sync"
)

const maxSize = 46

// index contains []byte which cap is 1<<index
var caches [maxSize]sync.Pool

func init() {
	for i := 0; i < maxSize; i++ {
		size := 1 << i
		caches[i].New = func() interface{} {
			buf := make([]byte, 0, size)
			return buf
		}
	}
}

// calculates which pool to get from
func calcIndex(size int) int {
	if size == 0 {
		return 0
	}
	if isPowerOfTwo(size) {
		return bsr(size)
	}
	return bsr(size) + 1
}

// Malloc supports one or two integer argument.
// The size specifies the length of the returned slice, which means len(ret) == size.
// A second integer argument may be provided to specify the minimum capacity, which means cap(ret) >= cap.
func Malloc(size int, capacity ...int) []byte {
	if len(capacity) > 1 {
		panic("too many arguments to Malloc")
	}
	if size < 0 {
		// uint(-1) feeds bits.Len → calcIndex → an out-of-range slice index
		// into `caches`. Reject explicitly instead of producing a panic
		// from deep inside the pool lookup.
		panic("mbuffer: negative size")
	}
	var c = size
	if len(capacity) > 0 && capacity[0] > size {
		c = capacity[0]
	}
	idx := calcIndex(c)
	if idx >= maxSize {
		// Requested capacity exceeds the largest cached bucket; allocate
		// directly so callers can still get a backing array.
		return make([]byte, size)
	}
	var ret = caches[idx].Get().([]byte)
	ret = ret[:size]
	return ret
}

// Free should be called when the buf is no longer used.
func Free(buf []byte) {
	size := cap(buf)
	// `isPowerOfTwo(0)` returns true because (0 & -0) == 0; combined with
	// bsr(0) == -1 the original Free panicked on a zero-capacity slice.
	// Reject explicitly here.
	if size <= 0 {
		return
	}
	if !isPowerOfTwo(size) {
		return
	}
	buf = buf[:0]
	caches[bsr(size)].Put(buf)
}
