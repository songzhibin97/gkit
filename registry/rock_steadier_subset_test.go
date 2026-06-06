package registry

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
)

// TestRockSteadierSubset_ConcurrentGetServicesNoRace guards the copy-on-write
// matrix change: GetServices iterates the inner []*int rows of the atomic.Value
// snapshot while the sentinel applies AddService/RemoveService. The old code
// mutated those rows in place (atomic.Value only protects the outer header), so
// a concurrent GetServices raced the writer. Run under -race; reverting
// addService/removeService to in-place mutation trips the detector.
func TestRockSteadierSubset_ConcurrentGetServicesNoRace(t *testing.T) {
	ctx := context.Background()
	clients := []int{0, 1, 2, 3}
	services := []int{10, 11, 12}
	r := NewRockSteadierSubset(ctx, clients, services, func() int64 { return 1 })
	defer r.Close()

	var wg sync.WaitGroup
	stop := int32(0)

	// Readers: spam GetServices across all clients.
	for c := range clients {
		client := clients[c]
		wg.Add(1)
		go func() {
			defer wg.Done()
			for atomic.LoadInt32(&stop) == 0 {
				_ = r.GetServices(client)
			}
		}()
	}

	// Writer: Add/Remove services (the sentinel applies them via copy-on-write).
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 100; i < 100+2000; i++ {
			_ = r.AddService(ctx, []int{i})
			_ = r.RemoveService(ctx, []int{i})
		}
		atomic.StoreInt32(&stop, 1)
	}()

	wg.Wait()
}

// TestRockSteadierSubset_CloseDuringSendNoPanic guards the Close() fix: the
// closed-flag check and the channel send in AddService/RemoveService are not
// atomic, so a sender could pass the flag check and then send. The old Close()
// did close(r.command), which made that late send panic ("send on closed
// channel"). Close() now only cancels the context and senders bail out on
// r.ctx.Done(). Reverting Close() to close(r.command) makes this test panic and
// crash the run.
func TestRockSteadierSubset_CloseDuringSendNoPanic(t *testing.T) {
	clients := []int{0, 1, 2, 3}
	services := []int{10, 11, 12}
	for iter := 0; iter < 50; iter++ {
		r := NewRockSteadierSubset(context.Background(), clients, services, func() int64 { return 1 })
		var wg sync.WaitGroup
		for g := 0; g < 8; g++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 100; i++ {
					// Must never panic regardless of when Close() lands.
					_ = r.AddService(context.Background(), []int{i})
					_ = r.RemoveService(context.Background(), []int{i})
				}
			}()
		}
		// Close concurrently with the in-flight senders.
		r.Close()
		wg.Wait()
	}
}
