package pool

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var errActiveRaceFactory = errors.New("controlled factory failure")

type activeRaceResource struct {
	shutdowns int32
}

func (r *activeRaceResource) Shutdown() error {
	atomic.AddInt32(&r.shutdowns, 1)
	return nil
}

func waitForAtomicAtLeast(t *testing.T, value *int64, want int64) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if atomic.LoadInt64(value) >= want {
			return
		}
		runtime.Gosched()
	}
	t.Fatalf("counter = %d, want at least %d", atomic.LoadInt64(value), want)
}

func updateAtomicMax(value *int64, candidate int64) {
	for {
		current := atomic.LoadInt64(value)
		if candidate <= current || atomic.CompareAndSwapInt64(value, current, candidate) {
			return
		}
	}
}

func TestListActiveCountConcurrentRelease(t *testing.T) {
	const workers = 8

	list := NewList(SetActive(1), SetIdle(0), SetWait(false, 0)).(*List)
	defer func() { _ = list.Shutdown() }()

	held := &activeRaceResource{}
	failureEntered := make(chan struct{})
	failureRelease := make(chan struct{})
	var factoryCalls int64
	var inFactory int64
	var maxInFactory int64
	list.New(func(context.Context) (IShutdown, error) {
		call := atomic.AddInt64(&factoryCalls, 1)
		inFlight := atomic.AddInt64(&inFactory, 1)
		updateAtomicMax(&maxInFactory, inFlight)
		defer atomic.AddInt64(&inFactory, -1)

		if call == 1 {
			return held, nil
		}
		if call == 2 {
			close(failureEntered)
			<-failureRelease
		}
		runtime.Gosched()
		return nil, errActiveRaceFactory
	})

	resource, err := list.Get(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if resource != held {
		t.Fatalf("first resource = %p, want held resource %p", resource, held)
	}

	start := make(chan struct{})
	ready := make(chan struct{}, workers)
	unexpected := make(chan error, workers)
	var attempts int64
	var stop int32
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			ready <- struct{}{}
			<-start
			for atomic.LoadInt32(&stop) == 0 {
				atomic.AddInt64(&attempts, 1)
				got, getErr := list.Get(context.Background())
				if got != nil {
					unexpected <- errors.New("failing factory returned a resource")
					return
				}
				if getErr != ErrPoolExhausted && !errors.Is(getErr, errActiveRaceFactory) {
					unexpected <- getErr
					return
				}
			}
		}()
	}
	for i := 0; i < workers; i++ {
		<-ready
	}
	close(start)

	// Keep active at the configured limit while every worker repeatedly executes
	// the capacity check, then force-close the held resource to start the
	// controlled factory-failure/release cycle.
	waitForAtomicAtLeast(t, &attempts, 1_000)
	putDone := make(chan error, 1)
	go func() {
		putDone <- list.Put(context.Background(), held, true)
	}()

	select {
	case <-failureEntered:
	case <-time.After(3 * time.Second):
		t.Fatal("factory failure did not enter")
	}
	beforeRelease := atomic.LoadInt64(&attempts)
	waitForAtomicAtLeast(t, &attempts, beforeRelease+1_000)
	close(failureRelease)
	waitForAtomicAtLeast(t, &factoryCalls, 200)

	atomic.StoreInt32(&stop, 1)
	wg.Wait()
	if err := <-putDone; err != nil {
		t.Fatalf("force-close Put: %v", err)
	}
	select {
	case err := <-unexpected:
		t.Fatal(err)
	default:
	}

	if got := atomic.LoadInt64(&maxInFactory); got > 1 {
		t.Fatalf("concurrent factory calls = %d, active limit = 1", got)
	}
	if got := atomic.LoadUint64(&list.active); got != 0 {
		t.Fatalf("active after all controlled factory failures = %d, want 0", got)
	}
	if got := atomic.LoadInt32(&held.shutdowns); got != 1 {
		t.Fatalf("force-closed resource Shutdown count = %d, want 1", got)
	}
}
