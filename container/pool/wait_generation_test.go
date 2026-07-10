package pool

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type waitGateContext struct {
	context.Context
	entered chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func newWaitGateContext(parent context.Context, release <-chan struct{}) *waitGateContext {
	return &waitGateContext{
		Context: parent,
		entered: make(chan struct{}),
		release: release,
	}
}

func (c *waitGateContext) enter() {
	c.once.Do(func() { close(c.entered) })
	<-c.release
}

func (c *waitGateContext) Deadline() (time.Time, bool) {
	c.enter()
	return c.Context.Deadline()
}

func (c *waitGateContext) Done() <-chan struct{} {
	c.enter()
	return c.Context.Done()
}

type waitTestResource struct {
	id        int64
	shutdowns int32
}

func (r *waitTestResource) Shutdown() error {
	atomic.AddInt32(&r.shutdowns, 1)
	return nil
}

type waitGetResult struct {
	resource IShutdown
	err      error
}

func newWaitTestList(t *testing.T, active uint64, wait bool, waitTimeout time.Duration) *List {
	t.Helper()
	list := NewList(
		SetActive(active),
		SetIdle(active),
		SetIdleTimeout(0),
		SetWait(wait, waitTimeout),
	).(*List)
	var id int64
	list.New(func(context.Context) (IShutdown, error) {
		return &waitTestResource{id: atomic.AddInt64(&id, 1)}, nil
	})
	return list
}

func mustGetWaitResource(t *testing.T, list *List) IShutdown {
	t.Helper()
	resource, err := list.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return resource
}

func awaitWaitGet(t *testing.T, result <-chan waitGetResult) waitGetResult {
	t.Helper()
	select {
	case got := <-result:
		return got
	case <-time.After(2 * time.Second):
		t.Fatal("Get did not return")
		return waitGetResult{}
	}
}

func TestListWaitBroadcastCannotBeLost(t *testing.T) {
	t.Run("resources returned before select", func(t *testing.T) {
		list := newWaitTestList(t, 2, true, 500*time.Millisecond)
		defer func() { _ = list.Shutdown() }()
		held := []IShutdown{mustGetWaitResource(t, list), mustGetWaitResource(t, list)}

		releaseSelect := make(chan struct{})
		contexts := []*waitGateContext{
			newWaitGateContext(context.Background(), releaseSelect),
			newWaitGateContext(context.Background(), releaseSelect),
		}
		results := make(chan waitGetResult, len(contexts))
		for _, ctx := range contexts {
			go func(ctx context.Context) {
				resource, err := list.Get(ctx)
				results <- waitGetResult{resource: resource, err: err}
			}(ctx)
		}
		for _, ctx := range contexts {
			select {
			case <-ctx.entered:
			case <-time.After(time.Second):
				t.Fatal("waiter did not reach the pre-select gate")
			}
		}

		// Both waiters have passed the locked state check and are stopped before
		// select. Returning both resources now proves that a prior notification is
		// retained and broadcast rather than coalesced or dropped.
		for _, resource := range held {
			if err := list.Put(context.Background(), resource, false); err != nil {
				t.Fatal(err)
			}
		}
		close(releaseSelect)

		seen := make(map[IShutdown]bool, len(held))
		for range contexts {
			got := awaitWaitGet(t, results)
			if got.err != nil {
				t.Fatalf("waiting Get: %v", got.err)
			}
			seen[got.resource] = true
			if err := list.Put(context.Background(), got.resource, true); err != nil {
				t.Fatal(err)
			}
		}
		for _, resource := range held {
			if !seen[resource] {
				t.Fatalf("returned resource %p was not received by a waiter", resource)
			}
		}
	})

	t.Run("shutdown broadcast", func(t *testing.T) {
		list := newWaitTestList(t, 1, true, 500*time.Millisecond)
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

		if err := list.Shutdown(); err != nil {
			t.Fatal(err)
		}
		close(releaseSelect)
		got := awaitWaitGet(t, result)
		if got.resource != nil || got.err != ErrPoolClosed {
			t.Fatalf("Get after Shutdown = (%v, %v), want (nil, ErrPoolClosed)", got.resource, got.err)
		}
		if err := list.Put(context.Background(), held, true); err != nil {
			t.Fatal(err)
		}
	})
}

func TestListWaitWithoutPoolTimeoutUsesCallerContext(t *testing.T) {
	t.Run("resource return", func(t *testing.T) {
		list := newWaitTestList(t, 1, true, 0)
		defer func() { _ = list.Shutdown() }()
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

		if err := list.Put(context.Background(), held, false); err != nil {
			t.Fatal(err)
		}
		close(releaseSelect)
		got := awaitWaitGet(t, result)
		if got.err != nil || got.resource != held {
			t.Fatalf("Get after Put = (%v, %v), want held resource", got.resource, got.err)
		}
		if err := list.Put(context.Background(), got.resource, true); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("caller cancellation", func(t *testing.T) {
		list := newWaitTestList(t, 1, true, 0)
		defer func() { _ = list.Shutdown() }()
		held := mustGetWaitResource(t, list)
		base, cancel := context.WithCancel(context.Background())
		defer cancel()
		releaseSelect := make(chan struct{})
		ctx := newWaitGateContext(base, releaseSelect)
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
		close(releaseSelect)

		select {
		case got := <-result:
			t.Fatalf("Get returned before caller cancellation: (%v, %v)", got.resource, got.err)
		case <-time.After(50 * time.Millisecond):
		}
		cancel()
		got := awaitWaitGet(t, result)
		if got.resource != nil || !errors.Is(got.err, context.Canceled) {
			t.Fatalf("Get after caller cancel = (%v, %v), want (nil, context.Canceled)", got.resource, got.err)
		}
		if err := list.Put(context.Background(), held, true); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("wait false zero remains exhausted", func(t *testing.T) {
		list := newWaitTestList(t, 1, false, 0)
		defer func() { _ = list.Shutdown() }()
		held := mustGetWaitResource(t, list)
		resource, err := list.Get(context.Background())
		if resource != nil || err != ErrPoolExhausted {
			t.Fatalf("Get exhausted = (%v, %v), want (nil, ErrPoolExhausted)", resource, err)
		}
		if err := list.Put(context.Background(), held, true); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("positive timeout remains bounded", func(t *testing.T) {
		list := newWaitTestList(t, 1, false, 30*time.Millisecond)
		defer func() { _ = list.Shutdown() }()
		held := mustGetWaitResource(t, list)
		resource, err := list.Get(context.Background())
		if resource != nil || !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("Get after positive pool timeout = (%v, %v), want deadline exceeded", resource, err)
		}
		if err := list.Put(context.Background(), held, true); err != nil {
			t.Fatal(err)
		}
	})
}
