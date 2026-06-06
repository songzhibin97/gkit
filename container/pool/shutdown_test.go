package pool

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type countingShutdown struct {
	n *int32
}

func (c *countingShutdown) Shutdown() error {
	atomic.AddInt32(c.n, 1)
	return nil
}

// TestListShutdown_ShutsDownEachIdleOnce pins Shutdown's behaviour after the
// rewrite (snapshot the idle IShutdowns under the lock, Init the live list,
// then Shutdown off-lock): every idle resource is Shutdown exactly once, and a
// second Shutdown is an idempotent no-op returning ErrPoolClosed without
// touching any resource again. (The container/list value-copy the rewrite
// removed happens to traverse correctly read-only, so it is not separately
// distinguishable here; this guards the count/idempotency contract.)
func TestListShutdown_ShutsDownEachIdleOnce(t *testing.T) {
	pool := NewList(SetActive(4), SetIdle(4), SetIdleTimeout(90*time.Second), SetWait(false, 10*time.Millisecond))

	var mu sync.Mutex
	var counters []*int32
	pool.New(func(ctx context.Context) (IShutdown, error) {
		n := new(int32)
		mu.Lock()
		counters = append(counters, n)
		mu.Unlock()
		return &countingShutdown{n: n}, nil
	})

	// Get 4 resources, then return them all so they sit idle.
	conns := make([]IShutdown, 0, 4)
	for i := 0; i < 4; i++ {
		c, err := pool.Get(context.TODO())
		if err != nil {
			t.Fatalf("Get %d: %v", i, err)
		}
		conns = append(conns, c)
	}
	for i, c := range conns {
		if err := pool.Put(context.TODO(), c, false); err != nil {
			t.Fatalf("Put %d: %v", i, err)
		}
	}

	if err := pool.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(counters) == 0 {
		t.Fatal("no resources were created")
	}
	for i, n := range counters {
		if got := atomic.LoadInt32(n); got != 1 {
			t.Fatalf("idle resource %d Shutdown count = %d, want exactly 1", i, got)
		}
	}

	// Idempotent: a second Shutdown returns ErrPoolClosed and shuts nothing
	// down again.
	if err := pool.Shutdown(); err != ErrPoolClosed {
		t.Fatalf("second Shutdown err = %v, want ErrPoolClosed", err)
	}
	for i, n := range counters {
		if got := atomic.LoadInt32(n); got != 1 {
			t.Fatalf("resource %d Shutdown count after 2nd Shutdown = %d, want 1 (no double shutdown)", i, got)
		}
	}
}
