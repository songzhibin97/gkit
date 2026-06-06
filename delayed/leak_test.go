package delayed

import (
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

// funcDelayed is a Delayed whose Do runs an arbitrary func, for tests that
// need a task to do something (e.g. signal a channel). It carries a func and is
// therefore uncomparable — which also exercises the heap's no-== invariant.
type funcDelayed struct {
	exec int64
	do   func()
}

func (f funcDelayed) Do() {
	if f.do != nil {
		f.do()
	}
}
func (f funcDelayed) ExecTime() int64  { return f.exec }
func (f funcDelayed) Identify() string { return "funcDelayed" }

// TestAddDelayed_UncomparableDelayed guards the heap against comparing Delayed
// values with == (siftup/siftdown). funcDelayed carries a func and is therefore
// uncomparable; before the fix, AddDelayed panicked with "comparing
// uncomparable type" as soon as siftup had to move an element.
func TestAddDelayed_UncomparableDelayed(t *testing.T) {
	d := NewDispatchingDelayed(func(o interface{}) {
		if dd, ok := o.(*DispatchingDelayed); ok {
			dd.signal = nil
		}
	})
	defer d.Close()
	// Descending exec times force siftup to move elements (the == site).
	for i := 0; i < 16; i++ {
		d.AddDelayed(funcDelayed{exec: int64(100 - i), do: func() {}})
	}
}

// TestNewDispatchingDelayed_NoPoolReplacement covers C15: the constructor must
// create its goroutine pool exactly once, after options are applied. The old
// code allocated a Worker=1 pool inside the struct literal and, when the user
// supplied Worker != 1, reassigned the field WITHOUT shutting the first pool
// down — leaking that pool's preloaded idle goroutine on every construction
// (goroutine.NewGoroutine eagerly starts SetIdle goroutines that block until
// Shutdown is called).
//
// A single construction leaks only one goroutine, which is within scheduler
// noise, so we amplify: construct+Close many times and assert the goroutine
// count returns to baseline. A per-construction pool leak grows without bound
// and trips the bound; the single-pool fix stays flat.
func TestNewDispatchingDelayed_NoPoolReplacement(t *testing.T) {
	opt := func(o interface{}) {
		if dd, ok := o.(*DispatchingDelayed); ok {
			dd.Worker = 4
			dd.signal = nil // don't register process-wide SIGINT handlers in tests
		}
	}

	// Sanity: the resolved Worker is honoured (not silently reset).
	d0 := NewDispatchingDelayed(opt)
	if d0.Worker != 4 {
		t.Fatalf("Worker = %d, want 4", d0.Worker)
	}
	if err := d0.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	baseline := runtime.NumGoroutine()

	const iterations = 40
	for i := 0; i < iterations; i++ {
		d := NewDispatchingDelayed(opt)
		if err := d.Close(); err != nil {
			t.Fatalf("Close (iter %d): %v", i, err)
		}
	}

	// Let the closed instances' goroutines wind down, then assert no growth.
	const tolerance = 8
	deadline := time.Now().Add(3 * time.Second)
	var got int
	for {
		runtime.GC()
		got = runtime.NumGoroutine()
		if got <= baseline+tolerance || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got > baseline+tolerance {
		t.Fatalf("goroutine leak after %d construct/Close cycles: baseline=%d got=%d (>%d); a discarded pool is being leaked per construction",
			iterations, baseline, got, baseline+tolerance)
	}
}

// TestClose_DropsPendingTasks documents drop-on-close: a pending (not-yet-due,
// so never submitted) task is dropped when Close is called, not force-run.
// Under the old "drain remaining on close" behaviour it would have executed.
func TestClose_DropsPendingTasks(t *testing.T) {
	d := NewDispatchingDelayed(func(o interface{}) {
		if dd, ok := o.(*DispatchingDelayed); ok {
			dd.signal = nil
			dd.checkTime = 5 * time.Millisecond
		}
	})

	var ran int32
	// Future-dated -> never due -> stays pending until Close drops it.
	d.AddDelayed(funcDelayed{exec: time.Now().Add(time.Hour).Unix(), do: func() {
		atomic.StoreInt32(&ran, 1)
	}})
	time.Sleep(50 * time.Millisecond) // a few sentinel ticks; the task stays pending

	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	time.Sleep(50 * time.Millisecond)
	if atomic.LoadInt32(&ran) != 0 {
		t.Fatal("pending future-dated task ran on Close; expected drop-on-close")
	}
}

// TestClose_UnblocksStuckSubmit guards the deadlock path: the sentinel may be
// wedged in the normal-tick submit (unbuffered send, all workers busy) when
// Close fires. Close cancels closeCtx so the wedged AddTaskN returns instead of
// blocking, letting the sentinel reach pool.Shutdown(); the wedged task is
// dropped. With a blocking AddTask (or no closeCancel), the sentinel stays
// wedged until the worker frees and then RUNS the wedged task — so we detect
// the regression by whether that task ran.
func TestClose_UnblocksStuckSubmit(t *testing.T) {
	d := NewDispatchingDelayed(func(o interface{}) {
		if dd, ok := o.(*DispatchingDelayed); ok {
			dd.Worker = 1
			dd.signal = nil
			dd.checkTime = 5 * time.Millisecond
		}
	})

	now := time.Now().Unix()
	occupied := make(chan struct{})
	block := make(chan struct{})
	// Due now: occupies the sole worker and stays busy until released.
	d.AddDelayed(funcDelayed{exec: now, do: func() {
		close(occupied)
		<-block
	}})
	// Due now: the sentinel wedges trying to submit this (worker busy).
	var wedgedRan int32
	d.AddDelayed(funcDelayed{exec: now, do: func() {
		atomic.StoreInt32(&wedgedRan, 1)
	}})

	<-occupied
	time.Sleep(50 * time.Millisecond) // let the sentinel wedge in the submit

	closed := make(chan struct{})
	go func() { _ = d.Close(); close(closed) }()
	time.Sleep(50 * time.Millisecond) // give Close time to cancel closeCtx (drops the wedged task)

	close(block) // release the worker
	select {
	case <-closed:
	case <-time.After(15 * time.Second):
		t.Fatal("Close() hung after releasing the worker")
	}

	time.Sleep(50 * time.Millisecond) // let a (buggy) wedged submit run if it would
	if atomic.LoadInt32(&wedgedRan) != 0 {
		t.Fatal("a task the sentinel was wedged on ran anyway; closeCtx cancel should have dropped it (regression: AddTask instead of AddTaskN, or missing closeCancel)")
	}
}

// TestClose_ClearsPendingTasks asserts Close releases the references to dropped
// pending tasks (d.delays), so a closed dispatcher doesn't retain every queued
// closure — this is a leak-cleanup PR. The clear in the sentinel's close branch
// happens-before sentinelDone, so it's visible once Close() returns.
func TestClose_ClearsPendingTasks(t *testing.T) {
	d := NewDispatchingDelayed(func(o interface{}) {
		if dd, ok := o.(*DispatchingDelayed); ok {
			dd.signal = nil
			dd.checkTime = 5 * time.Millisecond
		}
	})
	for i := 0; i < 8; i++ {
		// Future-dated so they stay pending (never submitted) until Close.
		d.AddDelayed(funcDelayed{exec: time.Now().Add(time.Hour).Unix(), do: func() {}})
	}
	if err := d.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	d.RLock()
	n := len(d.delays)
	d.RUnlock()
	if n != 0 {
		t.Fatalf("closed dispatcher retains %d pending tasks; expected d.delays cleared", n)
	}
}

// TestAddDelayed_NoAppendAfterClose covers the append-after-close TOCTOU:
// AddDelayed passes the atomic isClose pre-check, then Close sets isClose and
// the sentinel clears d.delays, then AddDelayed appends under the lock — leaking
// a task into a closed dispatcher. The re-check of isClose under the lock
// prevents it. We create the window deterministically: hold the lock so an
// in-flight AddDelayed parks just past its pre-check, then simulate Close's
// isClose=1 + the sentinel's clear, then release. With the re-check the adder
// must not append; removing it makes the append land (mutation-verified).
func TestAddDelayed_NoAppendAfterClose(t *testing.T) {
	d := NewDispatchingDelayed(func(o interface{}) {
		if dd, ok := o.(*DispatchingDelayed); ok {
			dd.signal = nil
			dd.checkTime = time.Hour // never tick
		}
	})

	d.Lock() // hold the write lock so the adder parks on it
	addReturned := make(chan struct{})
	go func() {
		// isClose is still 0 -> passes the atomic pre-check, then blocks on Lock().
		d.AddDelayed(funcDelayed{exec: time.Now().Add(time.Hour).Unix(), do: func() {}})
		close(addReturned)
	}()
	time.Sleep(50 * time.Millisecond) // adder is now parked on the lock, past its pre-check

	// Simulate Close() setting isClose and the sentinel clearing d.delays while
	// the adder waits (we hold the lock, so this is safe).
	atomic.StoreInt32(&d.isClose, 1)
	d.delays = nil
	d.Unlock() // adder proceeds: the under-lock re-check must reject the append

	<-addReturned
	d.RLock()
	n := len(d.delays)
	d.RUnlock()
	if n != 0 {
		t.Fatalf("AddDelayed appended to a closed/cleared dispatcher: %d (append-after-close)", n)
	}

	// cleanup
	atomic.StoreInt32(&d.isClose, 0)
	_ = d.Close()
}
