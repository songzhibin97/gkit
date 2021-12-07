package mutex

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/internal/clock"
)

func Test_Mutex_TryLock(t *testing.T) {
	var m Mutex
	m.Lock()
	clock.Sleep(time.Second)
	if m.TryLock() {
		t.Error("TryLock get lock error")
	}
	m.Unlock()
	if !m.TryLock() {
		t.Error("TryLock get lock error")
	}
	m.Unlock()
}

func utTriableMutexConcurrent(t *testing.T) {
	m := &Mutex{}
	cnt := int32(0)
	wg := &sync.WaitGroup{}
	wg.Add(1000)
	for i := 0; i < 1000; i++ {
		go func(tm *Mutex, wgi *sync.WaitGroup, cntPtr *int32, t *testing.T) {
			for {
				if tm.TryLock() {
					*cntPtr = *cntPtr + 1
					tm.Unlock()
					wgi.Done()
					break
				} else {
					runtime.Gosched()
				}
			}
		}(m, wg, &cnt, t)
	}
	wg.Wait()
	// fmt.Println("count=", cnt)
	if cnt != 1000 {
		t.Error("count error concurrency")
	}
}

func Test_Mutex_TryLock_Concurrent(t *testing.T) {
	utTriableMutexConcurrent(t)
}

func Benchmark_Mutex_TryLock(b *testing.B) {
	for n := 0; n < b.N; n++ {
		utTriableMutexConcurrent(nil)
	}
}
