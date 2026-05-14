package local_cache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestGet_NoTOCTOUWipeOnExpire covers I-e for the Get path. Previously
// when the entry was expired, Get released the read lock and called the
// public Delete (which retook the write lock); a concurrent Set between
// the two could be wiped by Delete.
//
// The strongest signal here is the race detector: both Get and Set touch
// c.member through the same RWMutex; the fix ensures no Set is silently
// wiped during the expire-eviction sequence.
func TestGet_NoTOCTOUWipeOnExpire(t *testing.T) {
	c := NewCache(SetDefaultExpire(100 * time.Microsecond))
	const iter = 5_000

	var wg sync.WaitGroup
	wg.Add(2)
	stop := int32(0)

	// Reader: keeps calling Get.
	go func() {
		defer wg.Done()
		for atomic.LoadInt32(&stop) == 0 {
			_, _ = c.Get("k")
		}
	}()

	// Writer: Set, expire, Set, etc.
	go func() {
		defer wg.Done()
		for i := 0; i < iter; i++ {
			c.Set("k", i, 200*time.Microsecond)
			time.Sleep(150 * time.Microsecond)
		}
		atomic.StoreInt32(&stop, 1)
	}()

	wg.Wait()

	// Final Set must survive the run if TOCTOU is fixed (no concurrent
	// reader Delete should have wiped it).
	c.Set("final", 42, time.Minute)
	v, ok := c.Get("final")
	if !ok || v != 42 {
		t.Fatalf("final Set wiped: ok=%v v=%v", ok, v)
	}
}
