package local_cache

import (
	"testing"
	"time"
)

// TestCapture_ReentrantNoDeadlock guards that the expire-eviction paths run the
// capture (eviction) callback OFF the write lock. A callback that re-enters the
// cache (here, Set) would otherwise deadlock on the non-reentrant RWMutex.
// Reverting any eviction path to capture under the lock (e.g. Get/Increment
// calling _delete instead of expireEvictUnlock) deadlocks this test.
func TestCapture_ReentrantNoDeadlock(t *testing.T) {
	var c Cache
	c = NewCache(SetCapture(func(k string, v interface{}) {
		// Re-enter the cache from inside the eviction callback.
		c.Set("reentry", 1, time.Minute)
	}))

	done := make(chan struct{})
	go func() {
		defer close(done)
		// Each path: install an already-expired entry, then access it so the
		// path evicts it and invokes capture (which re-enters via Set).
		c.Set("g", 0, time.Nanosecond)
		_, _ = c.Get("g")

		c.Set("ge", 0, time.Nanosecond)
		_, _, _ = c.GetWithExpire("ge")

		c.Set("i", 0, time.Nanosecond)
		_ = c.Increment("i", 1)

		c.Set("if", 0, time.Nanosecond)
		_ = c.IncrementFloat("if", 1)
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("re-entrant capture callback deadlocked — an eviction path ran capture under the cache lock")
	}
}
