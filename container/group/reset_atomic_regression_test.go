package group

import (
	"runtime"
	"testing"
	"time"
)

func TestGroupResetIsAtomicWithConcurrentGet(t *testing.T) {
	g := NewGroup(func() interface{} { return "old" }).(*Group)
	if got := g.Get("key"); got != "old" {
		t.Fatalf("initial Get() = %v, want old", got)
	}

	// Hold a reader while ReSet queues for the write lock, then queue another
	// reader behind it. RWMutex releases that reader when ReSet unlocks, so it
	// observes the exact state published by ReSet's critical section.
	g.RLock()
	resetStarted := make(chan struct{})
	resetDone := make(chan struct{})
	go func() {
		close(resetStarted)
		g.ReSet(func() interface{} { return "new" })
		close(resetDone)
	}()
	<-resetStarted

	deadline := time.Now().Add(time.Second)
	for g.TryRLock() {
		g.RUnlock()
		if time.Now().After(deadline) {
			g.RUnlock()
			t.Fatal("ReSet did not queue for the group lock")
		}
		runtime.Gosched()
	}

	type observedState struct {
		factoryValue interface{}
		cachedValue  interface{}
		cached       bool
	}
	observed := make(chan observedState, 1)
	observerStarted := make(chan struct{})
	go func() {
		close(observerStarted)
		g.RLock()
		cachedValue, cached := g.objs["key"]
		observed <- observedState{
			factoryValue: g.f(),
			cachedValue:  cachedValue,
			cached:       cached,
		}
		g.RUnlock()
	}()
	<-observerStarted
	// Give the observer an opportunity to block behind the queued writer before
	// releasing this reader. Without that ordering it would only observe the
	// post-ReSet state and would not exercise the publication boundary.
	for i := 0; i < 100; i++ {
		runtime.Gosched()
	}

	g.RUnlock()

	state := <-observed
	if state.factoryValue != "new" {
		t.Fatalf("published factory returned %v, want new", state.factoryValue)
	}
	if state.cached {
		t.Fatalf("new factory was visible with stale cache value %v", state.cachedValue)
	}
	select {
	case <-resetDone:
	case <-time.After(time.Second):
		t.Fatal("ReSet did not return")
	}

	if got := g.Get("key"); got != "new" {
		t.Fatalf("Get() after ReSet = %v, want new", got)
	}
}
