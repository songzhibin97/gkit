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

package skipset

import (
	"math"
	"sync"
	"testing"
)

type floatSetContract struct {
	add      func(float64) bool
	contains func(float64) bool
	remove   func(float64) bool
	length   func() int
	rangeAll func() []float64
}

func floatSetCases() []struct {
	name       string
	descending bool
	newSet     func() floatSetContract
} {
	return []struct {
		name       string
		descending bool
		newSet     func() floatSetContract
	}{
		{"float32 ascending", false, func() floatSetContract {
			s := NewFloat32()
			return float32SetContract(s.Add, s.Contains, s.Remove, s.Len, s.Range)
		}},
		{"float32 descending", true, func() floatSetContract {
			s := NewFloat32Desc()
			return float32SetContract(s.Add, s.Contains, s.Remove, s.Len, s.Range)
		}},
		{"float64 ascending", false, func() floatSetContract {
			s := NewFloat64()
			return float64SetContract(s.Add, s.Contains, s.Remove, s.Len, s.Range)
		}},
		{"float64 descending", true, func() floatSetContract {
			s := NewFloat64Desc()
			return float64SetContract(s.Add, s.Contains, s.Remove, s.Len, s.Range)
		}},
	}
}

func TestFloatSetsRejectNaN(t *testing.T) {
	for _, tc := range floatSetCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := tc.newSet()
			nan := math.NaN()

			if s.add(nan) {
				t.Fatal("Add(NaN) = true, want false")
			}
			if s.length() != 0 {
				t.Fatalf("Len() = %d after Add(NaN), want 0", s.length())
			}
			if s.contains(nan) {
				t.Fatal("Contains(NaN) = true, want false")
			}
			if s.remove(nan) {
				t.Fatal("Remove(NaN) = true, want false")
			}
			for _, value := range s.rangeAll() {
				if math.IsNaN(value) {
					t.Fatal("Range returned NaN")
				}
			}

			const goroutines = 32
			results := make([]bool, goroutines)
			var wg sync.WaitGroup
			wg.Add(goroutines)
			for i := range results {
				go func(i int) {
					defer wg.Done()
					results[i] = s.add(nan)
				}(i)
			}
			wg.Wait()
			for i, added := range results {
				if added {
					t.Fatalf("concurrent Add(NaN) call %d = true, want false", i)
				}
			}
			if s.length() != 0 {
				t.Fatalf("Len() = %d after concurrent Add(NaN), want 0", s.length())
			}
		})
	}
}

func TestFloatSetsOrderSpecialValues(t *testing.T) {
	for _, tc := range floatSetCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			s := tc.newSet()
			for _, value := range []float64{1, math.Inf(-1), math.Copysign(0, -1), math.Inf(1), -1} {
				if !s.add(value) {
					t.Fatalf("Add(%v) = false, want true", value)
				}
			}
			if s.add(0) {
				t.Fatal("Add(+0) = true after adding -0, want false")
			}

			want := []float64{math.Inf(-1), -1, 0, 1, math.Inf(1)}
			if tc.descending {
				want = []float64{math.Inf(1), 1, 0, -1, math.Inf(-1)}
			}
			assertFloatValues(t, s.rangeAll(), want)
		})
	}
}

func float32SetContract(
	add func(float32) bool,
	contains func(float32) bool,
	remove func(float32) bool,
	length func() int,
	rangeFn func(func(float32) bool),
) floatSetContract {
	return floatSetContract{
		add:      func(value float64) bool { return add(float32(value)) },
		contains: func(value float64) bool { return contains(float32(value)) },
		remove:   func(value float64) bool { return remove(float32(value)) },
		length:   length,
		rangeAll: func() (values []float64) {
			rangeFn(func(value float32) bool {
				values = append(values, float64(value))
				return true
			})
			return values
		},
	}
}

func float64SetContract(
	add func(float64) bool,
	contains func(float64) bool,
	remove func(float64) bool,
	length func() int,
	rangeFn func(func(float64) bool),
) floatSetContract {
	return floatSetContract{
		add:      add,
		contains: contains,
		remove:   remove,
		length:   length,
		rangeAll: func() (values []float64) {
			rangeFn(func(value float64) bool {
				values = append(values, value)
				return true
			})
			return values
		},
	}
}

func assertFloatValues(t *testing.T, got, want []float64) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("Range returned %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Range returned %v, want %v", got, want)
		}
	}
}
