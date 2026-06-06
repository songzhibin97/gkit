package broker

import (
	"context"
	"sync"
	"testing"
)

// TestBroker_RetryFieldAtomic covers I-q: previously `b.retry = false` in
// StopConsuming raced unsynchronised reads of `b.retry` from worker
// goroutines via GetRetry.
func TestBroker_RetryFieldAtomic(t *testing.T) {
	b := NewBroker(NewRegisteredTask(), context.Background(), SetRetry(true))
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 1_000; i++ {
			_ = b.GetRetry()
		}
	}()
	go func() {
		defer wg.Done()
		b.StopConsuming()
	}()
	wg.Wait()
	if b.GetRetry() {
		t.Fatal("retry should be false after StopConsuming")
	}
}
