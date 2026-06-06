package local_cache

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestGet_NoTOCTOUWipeOnExpire covers I-e for the Get path. Previously, when the
// entry was expired, Get released the read lock and called the public Delete
// (which retook the write lock) unconditionally; a concurrent Set that landed
// in that window installed a fresh value that Delete then wiped. This is a
// logical lost-update, NOT a data race (both paths are mutex-protected), so the
// race detector cannot see it.
//
// We expose it directly: the writer makes "k" expire (so a concurrent reader
// enters the expire-eviction path), then installs a fresh long-lived value and
// verifies it is still present. A reader eviction that does not re-check expiry
// under the write lock wipes that fresh value. Reverting the re-check fails this
// test.
func TestGet_NoTOCTOUWipeOnExpire(t *testing.T) {
	c := NewCache()

	var wiped int32
	stop := int32(0)
	var wg sync.WaitGroup
	wg.Add(2)

	// Reader: Get("k") tightly; runs the expire-eviction path when k is expired.
	go func() {
		defer wg.Done()
		for atomic.LoadInt32(&stop) == 0 {
			_, _ = c.Get("k")
		}
	}()

	// Writer: expire k, then install a fresh value and confirm it survives.
	go func() {
		defer wg.Done()
		for i := 1; i <= 300_000; i++ {
			c.Set("k", 0, time.Nanosecond) // expires immediately -> reader evicts
			c.Set("k", i, time.Hour)       // fresh, long-lived
			if v, ok := c.Get("k"); !ok || v != i {
				atomic.StoreInt32(&wiped, 1)
				break
			}
		}
		atomic.StoreInt32(&stop, 1)
	}()

	wg.Wait()

	if atomic.LoadInt32(&wiped) == 1 {
		t.Fatal("a fresh Set was wiped by a concurrent reader's stale expire-eviction (TOCTOU lost update)")
	}
}
