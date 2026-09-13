package syncx

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

// poolTestObject tracks exclusive ownership independently of the pool internals.
type poolTestObject struct {
	owned int32
	value int
}

func TestSyncXPool(t *testing.T) {
	var pool Pool
	pool.Put(nil)
	if got := pool.Get(); got != nil {
		t.Fatalf("empty pool Get = %v, want nil", got)
	}
	pool.New = func() interface{} { return &poolTestObject{value: 97} }
	for round := 0; round < 8; round++ {
		objects := make([]*poolTestObject, 1024)
		for i := range objects {
			obj, ok := pool.Get().(*poolTestObject)
			if !ok || obj == nil {
				t.Fatal("Get did not return a constructed object")
			}
			if !atomic.CompareAndSwapInt32(&obj.owned, 0, 1) {
				t.Fatal("Get returned an object still checked out")
			}
			if obj.value != 97 {
				t.Fatalf("object value = %d, want 97", obj.value)
			}
			objects[i] = obj
		}
		for _, obj := range objects {
			atomic.StoreInt32(&obj.owned, 0)
			pool.Put(obj)
		}
	}
}

func TestRaceSyncXPool(t *testing.T) {
	for _, noGC := range []bool{false, true} {
		t.Run(fmt.Sprintf("NoGC=%v", noGC), func(t *testing.T) {
			pool := Pool{New: func() interface{} { return &poolTestObject{value: 97} }, NoGC: noGC}
			const workers, batch = 8, 512
			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(workers)
			for worker := 0; worker < workers; worker++ {
				go func() {
					defer wg.Done()
					<-start
					for round := 0; round < 8; round++ {
						objects := make([]*poolTestObject, batch)
						for i := range objects {
							obj, ok := pool.Get().(*poolTestObject)
							if !ok || obj == nil {
								t.Error("Get did not return a constructed object")
								return
							}
							if !atomic.CompareAndSwapInt32(&obj.owned, 0, 1) {
								t.Error("Get returned an object owned by another checkout")
								return
							}
							if obj.value != 97 {
								t.Errorf("object value = %d, want 97", obj.value)
								return
							}
							objects[i] = obj
						}
						runtime.Gosched()
						for _, obj := range objects {
							atomic.StoreInt32(&obj.owned, 0)
							pool.Put(obj)
						}
					}
				}()
			}
			close(start)
			wg.Wait()
		})
	}
}

var p = 1024

func BenchmarkSyncPool(b *testing.B) {
	var pool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 1024)
		},
	}

	var bs = make([][]byte, p)

	// benchmark
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for i := 0; i < p; i++ {
			bs[i] = pool.Get().([]byte)
		}
		for i := 0; i < p; i++ {
			pool.Put(bs[i])
		}
	}
}

func BenchmarkSyncXPool(b *testing.B) {
	var pool = Pool{
		New: func() interface{} {
			return make([]byte, 1024)
		},
		NoGC: true,
	}

	var bs = make([][]byte, p)

	// benchmark
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for i := 0; i < p; i++ {
			bs[i] = pool.Get().([]byte)
		}
		for i := 0; i < p; i++ {
			pool.Put(bs[i])
		}
	}
}

func BenchmarkSyncPoolParallel(b *testing.B) {
	var pool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 1024)
		},
	}

	// benchmark
	b.ReportAllocs()
	b.SetParallelism(16)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var bs = make([][]byte, p)
		for pb.Next() {
			for i := 0; i < p; i++ {
				bs[i] = pool.Get().([]byte)
			}
			for i := 0; i < p; i++ {
				pool.Put(bs[i])
			}
		}
	})
}

func BenchmarkSyncXPoolParallel(b *testing.B) {
	var pool = Pool{
		New: func() interface{} {
			return make([]byte, 1024)
		},
		NoGC: true,
	}

	// benchmark
	b.ReportAllocs()
	b.SetParallelism(16)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var bs = make([][]byte, p)
		for pb.Next() {
			for i := 0; i < p; i++ {
				bs[i] = pool.Get().([]byte)
			}
			for i := 0; i < p; i++ {
				pool.Put(bs[i])
			}
		}
	})
}

func BenchmarkSyncPoolParallel1(b *testing.B) {
	var pool = sync.Pool{
		New: func() interface{} {
			return make([]byte, 1024)
		},
	}

	// benchmark
	b.ReportAllocs()
	b.SetParallelism(16)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var bs []byte
		for pb.Next() {
			bs = pool.Get().([]byte)
			pool.Put(bs)
		}
	})
}

func BenchmarkSyncXPoolParallel1(b *testing.B) {
	var pool = Pool{
		New: func() interface{} {
			return make([]byte, 1024)
		},
		NoGC: true,
	}

	// benchmark
	b.ReportAllocs()
	b.SetParallelism(16)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		var bs []byte
		for pb.Next() {
			bs = pool.Get().([]byte)
			pool.Put(bs)
		}
	})
}
