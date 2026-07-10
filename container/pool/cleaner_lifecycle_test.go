package pool

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type cleanerLifecycleResource struct {
	id        int64
	started   chan struct{}
	unblock   <-chan struct{}
	once      sync.Once
	shutdowns int32
}

func newCleanerLifecycleResource(id int64, unblock <-chan struct{}) *cleanerLifecycleResource {
	return &cleanerLifecycleResource{
		id:      id,
		started: make(chan struct{}),
		unblock: unblock,
	}
}

func (r *cleanerLifecycleResource) Shutdown() error {
	atomic.AddInt32(&r.shutdowns, 1)
	r.once.Do(func() { close(r.started) })
	if r.unblock != nil {
		<-r.unblock
	}
	return nil
}

func cleanupCleanerLifecycleList(t *testing.T, list *List) {
	t.Helper()
	t.Cleanup(func() {
		if err := list.Shutdown(); err != nil && err != ErrPoolClosed {
			t.Errorf("cleanup Shutdown: %v", err)
		}
	})
}

func ageCleanerLifecycleIdle(t *testing.T, list *List, age time.Duration) {
	t.Helper()
	list.mu.Lock()
	entry := list.idles.Back()
	if entry == nil {
		list.mu.Unlock()
		t.Fatal("pool has no idle resource to age")
	}
	idle := entry.Value.(item)
	idle.createdAt = nowFunc().Add(-age)
	entry.Value = idle
	list.mu.Unlock()
}

func mustGetCleanerLifecycleResource(t *testing.T, list *List) *cleanerLifecycleResource {
	t.Helper()
	resource, err := list.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	return resource.(*cleanerLifecycleResource)
}

func waitForCleanerLifecycleShutdown(t *testing.T, resource *cleanerLifecycleResource) {
	t.Helper()
	select {
	case <-resource.started:
	case <-time.After(2 * time.Second):
		t.Fatalf("resource %d was not cleaned", resource.id)
	}
	if got := atomic.LoadInt32(&resource.shutdowns); got != 1 {
		t.Fatalf("resource %d Shutdown count = %d, want 1", resource.id, got)
	}
}

func waitForCleanerLifecycleClosed(t *testing.T, list *List) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadUint32(&list.closed) == 1 {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("pool did not enter the closed state")
}

func TestListCleanerFollowsReloadedIdleTimeout(t *testing.T) {
	list := NewList(
		SetActive(1),
		SetIdle(1),
		SetIdleTimeout(0),
		SetWait(false, 0),
	).(*List)
	cleanupCleanerLifecycleList(t, list)

	var nextID int64
	list.New(func(context.Context) (IShutdown, error) {
		return newCleanerLifecycleResource(atomic.AddInt64(&nextID, 1), nil), nil
	})

	// A cleaner must remain reloadable even when it was disabled at construction.
	first := mustGetCleanerLifecycleResource(t, list)
	if err := list.Put(context.Background(), first, false); err != nil {
		t.Fatal(err)
	}
	ageCleanerLifecycleIdle(t, list, 2*time.Hour)
	list.Reload(SetIdleTimeout(time.Hour))
	waitForCleanerLifecycleShutdown(t, first)

	// Disabling cleanup must use the latest configuration. The watchdog only
	// guards against an erroneous close; expiry itself is controlled by the idle
	// timestamp, and Get proves that the same resource remains available.
	second := mustGetCleanerLifecycleResource(t, list)
	if err := list.Put(context.Background(), second, false); err != nil {
		t.Fatal(err)
	}
	list.Reload(SetIdleTimeout(0))
	ageCleanerLifecycleIdle(t, list, 2*time.Hour)
	select {
	case <-second.started:
		t.Fatal("resource was cleaned while idle cleanup was disabled")
	case <-time.After(2 * minDuration):
	}
	got := mustGetCleanerLifecycleResource(t, list)
	if got != second {
		t.Fatalf("Get while cleanup disabled returned resource %d, want %d", got.id, second.id)
	}
	if err := list.Put(context.Background(), got, false); err != nil {
		t.Fatal(err)
	}

	// Re-enabling cleanup must immediately apply to resources that became stale
	// while disabled.
	ageCleanerLifecycleIdle(t, list, 2*time.Hour)
	list.Reload(SetIdleTimeout(time.Hour))
	waitForCleanerLifecycleShutdown(t, second)

	// A positive period change also uses the newest value: a long timeout keeps
	// the resource, then shortening it cleans the now-expired resource.
	third := mustGetCleanerLifecycleResource(t, list)
	if err := list.Put(context.Background(), third, false); err != nil {
		t.Fatal(err)
	}
	list.Reload(SetIdleTimeout(10 * time.Hour))
	ageCleanerLifecycleIdle(t, list, 2*time.Hour)
	got = mustGetCleanerLifecycleResource(t, list)
	if got != third {
		t.Fatalf("Get under longer timeout returned resource %d, want %d", got.id, third.id)
	}
	if err := list.Put(context.Background(), got, false); err != nil {
		t.Fatal(err)
	}
	ageCleanerLifecycleIdle(t, list, 2*time.Hour)
	list.Reload(SetIdleTimeout(time.Hour))
	waitForCleanerLifecycleShutdown(t, third)
}

func TestListShutdownStopsCleanerAndWakesWaiters(t *testing.T) {
	t.Run("waits for cleaner completion", func(t *testing.T) {
		release := make(chan struct{})
		var releaseOnce sync.Once
		unblock := func() { releaseOnce.Do(func() { close(release) }) }

		list := NewList(
			SetActive(1),
			SetIdle(1),
			SetIdleTimeout(time.Hour),
			SetWait(true, 0),
		).(*List)
		cleanupCleanerLifecycleList(t, list)
		t.Cleanup(unblock)

		resource := newCleanerLifecycleResource(1, release)
		list.New(func(context.Context) (IShutdown, error) { return resource, nil })
		if got := mustGetCleanerLifecycleResource(t, list); got != resource {
			t.Fatalf("Get returned %p, want %p", got, resource)
		}
		if err := list.Put(context.Background(), resource, false); err != nil {
			t.Fatal(err)
		}

		// Make the idle entry expired without waiting for wall-clock time. Init's
		// wake then deterministically drives the cleaner into resource Shutdown.
		ageCleanerLifecycleIdle(t, list, 2*time.Hour)
		list.Init(time.Nanosecond)

		select {
		case <-resource.started:
		case <-time.After(2 * time.Second):
			t.Fatal("cleaner did not begin resource Shutdown")
		}

		shutdownResult := make(chan error, 1)
		go func() { shutdownResult <- list.Shutdown() }()
		waitForCleanerLifecycleClosed(t, list)
		select {
		case err := <-shutdownResult:
			t.Fatalf("Shutdown returned before cleaner completed: %v", err)
		case <-time.After(minDuration):
		}

		unblock()
		select {
		case err := <-shutdownResult:
			if err != nil {
				t.Fatalf("Shutdown: %v", err)
			}
		case <-time.After(2 * time.Second):
			t.Fatal("Shutdown did not return after cleaner completed")
		}
		if got := atomic.LoadInt32(&resource.shutdowns); got != 1 {
			t.Fatalf("cleaned resource Shutdown count = %d, want 1", got)
		}
		if err := list.Shutdown(); err != ErrPoolClosed {
			t.Fatalf("second Shutdown err = %v, want ErrPoolClosed", err)
		}
	})

	t.Run("stops disabled cleaner and broadcasts close", func(t *testing.T) {
		list := newWaitTestList(t, 1, true, 0)
		cleanupCleanerLifecycleList(t, list)
		held := mustGetWaitResource(t, list)

		releaseSelect := make(chan struct{})
		ctx := newWaitGateContext(context.Background(), releaseSelect)
		result := make(chan waitGetResult, 1)
		go func() {
			resource, err := list.Get(ctx)
			result <- waitGetResult{resource: resource, err: err}
		}()
		select {
		case <-ctx.entered:
		case <-time.After(time.Second):
			t.Fatal("waiter did not reach the pre-select gate")
		}

		shutdownResult := make(chan error, 1)
		go func() { shutdownResult <- list.Shutdown() }()
		select {
		case err := <-shutdownResult:
			if err != nil {
				t.Fatalf("Shutdown: %v", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Shutdown did not stop the disabled cleaner")
		}
		close(releaseSelect)
		got := awaitWaitGet(t, result)
		if got.resource != nil || got.err != ErrPoolClosed {
			t.Fatalf("waiting Get after Shutdown = (%v, %v), want (nil, ErrPoolClosed)", got.resource, got.err)
		}

		// Reload after shutdown must not send to a lifecycle channel that was
		// closed as part of stopping the cleaner.
		list.Reload(SetIdleTimeout(time.Second))
		if err := list.Shutdown(); err != ErrPoolClosed {
			t.Fatalf("second Shutdown err = %v, want ErrPoolClosed", err)
		}
		if err := list.Put(context.Background(), held, true); err != nil {
			t.Fatal(err)
		}
	})
}
