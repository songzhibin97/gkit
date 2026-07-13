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

package skipmap

import (
	"math"
	"sync"
	"testing"
)

type floatMapContract struct {
	store           func(float64, interface{})
	load            func(float64) (interface{}, bool)
	loadOrStore     func(float64, interface{}) (interface{}, bool)
	loadOrStoreLazy func(float64, func() interface{}) (interface{}, bool)
	delete          func(float64) bool
	length          func() int
	rangeKeys       func() []float64
}

func floatMapCases() []struct {
	name       string
	descending bool
	newMap     func() floatMapContract
} {
	return []struct {
		name       string
		descending bool
		newMap     func() floatMapContract
	}{
		{"float32 ascending", false, func() floatMapContract {
			m := NewFloat32()
			return float32MapContract(m.Store, m.Load, m.LoadOrStore, m.LoadOrStoreLazy, m.Delete, m.Len, m.Range)
		}},
		{"float32 descending", true, func() floatMapContract {
			m := NewFloat32Desc()
			return float32MapContract(m.Store, m.Load, m.LoadOrStore, m.LoadOrStoreLazy, m.Delete, m.Len, m.Range)
		}},
		{"float64 ascending", false, func() floatMapContract {
			m := NewFloat64()
			return float64MapContract(m.Store, m.Load, m.LoadOrStore, m.LoadOrStoreLazy, m.Delete, m.Len, m.Range)
		}},
		{"float64 descending", true, func() floatMapContract {
			m := NewFloat64Desc()
			return float64MapContract(m.Store, m.Load, m.LoadOrStore, m.LoadOrStoreLazy, m.Delete, m.Len, m.Range)
		}},
	}
}

func TestFloatMapsRejectNaN(t *testing.T) {
	for _, tc := range floatMapCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := tc.newMap()
			nan := math.NaN()

			for i := 0; i < 3; i++ {
				m.store(nan, i)
			}
			if got := m.length(); got != 0 {
				t.Fatalf("Len() = %d after Store(NaN), want 0", got)
			}
			if value, ok := m.load(nan); ok || value != nil {
				t.Fatalf("Load(NaN) = (%v, %t), want (nil, false)", value, ok)
			}
			if m.delete(nan) {
				t.Fatal("Delete(NaN) = true, want false")
			}

			if actual, loaded := m.loadOrStore(nan, "value"); actual != nil || loaded {
				t.Fatalf("LoadOrStore(NaN) = (%v, %t), want (nil, false)", actual, loaded)
			}
			if got := m.length(); got != 0 {
				t.Fatalf("Len() = %d after LoadOrStore(NaN), want 0", got)
			}

			called := false
			actual, loaded := m.loadOrStoreLazy(nan, func() interface{} {
				called = true
				return "lazy"
			})
			if actual != nil || loaded {
				t.Fatalf("LoadOrStoreLazy(NaN) = (%v, %t), want (nil, false)", actual, loaded)
			}
			if called {
				t.Fatal("LoadOrStoreLazy(NaN) called f, want f not called")
			}
			if got := m.length(); got != 0 {
				t.Fatalf("Len() = %d after LoadOrStoreLazy(NaN), want 0", got)
			}

			for _, key := range m.rangeKeys() {
				if math.IsNaN(key) {
					t.Fatal("Range returned NaN key")
				}
			}

			const goroutines = 32
			var wg sync.WaitGroup
			wg.Add(goroutines)
			for i := 0; i < goroutines; i++ {
				go func(i int) {
					defer wg.Done()
					switch i % 3 {
					case 0:
						m.store(nan, i)
					case 1:
						m.loadOrStore(nan, i)
					default:
						m.loadOrStoreLazy(nan, func() interface{} { return i })
					}
				}(i)
			}
			wg.Wait()
			if got := m.length(); got != 0 {
				t.Fatalf("Len() = %d after concurrent NaN writes, want 0", got)
			}

			// A comparable key must keep working after NaN rejections.
			m.store(1.5, "kept")
			if value, ok := m.load(1.5); !ok || value != "kept" {
				t.Fatalf("Load(1.5) = (%v, %t), want (kept, true)", value, ok)
			}
			if got := m.length(); got != 1 {
				t.Fatalf("Len() = %d after Store(1.5), want 1", got)
			}
		})
	}
}

func TestFloatMapsOrderSpecialValues(t *testing.T) {
	for _, tc := range floatMapCases() {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := tc.newMap()
			for _, key := range []float64{1, math.Inf(-1), math.Copysign(0, -1), math.Inf(1), -1} {
				m.store(key, key)
			}
			if actual, loaded := m.loadOrStore(0, "zero"); !loaded {
				t.Fatalf("LoadOrStore(+0) = (%v, %t) after Store(-0), want loaded", actual, loaded)
			}
			if got := m.length(); got != 5 {
				t.Fatalf("Len() = %d, want 5; +0 and -0 must be the same key", got)
			}

			want := []float64{math.Inf(-1), -1, 0, 1, math.Inf(1)}
			if tc.descending {
				want = []float64{math.Inf(1), 1, 0, -1, math.Inf(-1)}
			}
			assertFloatKeys(t, m.rangeKeys(), want)
		})
	}
}

func float32MapContract(
	store func(float32, interface{}),
	load func(float32) (interface{}, bool),
	loadOrStore func(float32, interface{}) (interface{}, bool),
	loadOrStoreLazy func(float32, func() interface{}) (interface{}, bool),
	deleteFn func(float32) bool,
	length func() int,
	rangeFn func(func(float32, interface{}) bool),
) floatMapContract {
	return floatMapContract{
		store: func(key float64, value interface{}) { store(float32(key), value) },
		load:  func(key float64) (interface{}, bool) { return load(float32(key)) },
		loadOrStore: func(key float64, value interface{}) (interface{}, bool) {
			return loadOrStore(float32(key), value)
		},
		loadOrStoreLazy: func(key float64, f func() interface{}) (interface{}, bool) {
			return loadOrStoreLazy(float32(key), f)
		},
		delete: func(key float64) bool { return deleteFn(float32(key)) },
		length: length,
		rangeKeys: func() (keys []float64) {
			rangeFn(func(key float32, _ interface{}) bool {
				keys = append(keys, float64(key))
				return true
			})
			return keys
		},
	}
}

func float64MapContract(
	store func(float64, interface{}),
	load func(float64) (interface{}, bool),
	loadOrStore func(float64, interface{}) (interface{}, bool),
	loadOrStoreLazy func(float64, func() interface{}) (interface{}, bool),
	deleteFn func(float64) bool,
	length func() int,
	rangeFn func(func(float64, interface{}) bool),
) floatMapContract {
	return floatMapContract{
		store:           store,
		load:            load,
		loadOrStore:     loadOrStore,
		loadOrStoreLazy: loadOrStoreLazy,
		delete:          deleteFn,
		length:          length,
		rangeKeys: func() (keys []float64) {
			rangeFn(func(key float64, _ interface{}) bool {
				keys = append(keys, key)
				return true
			})
			return keys
		},
	}
}

func assertFloatKeys(t *testing.T, got, want []float64) {
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
