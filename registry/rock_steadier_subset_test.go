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
