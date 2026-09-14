package skipmap_test

import (
	"github.com/songzhibin97/gkit/structure/skipmap"
	"sync"
	"testing"
)

// Bounded public-API history check: no private node state or schedule hook.
// With no Delete calls, every completed write must be followed by a present key.
func TestPublicWriteThenLoad(t *testing.T) {
	for _, mode := range []string{"Store", "LoadOrStore", "LoadOrStoreLazy"} {
		t.Run(mode, func(t *testing.T) {
			const rounds = 5000
			for i := 0; i < rounds; i++ {
				m := skipmap.NewInt64()
				start := make(chan struct{})
				misses := make(chan string, 2)
				var wg sync.WaitGroup
				wg.Add(2)
				go func() {
					defer wg.Done()
					<-start
					m.Store(7, "first")
					if _, ok := m.Load(7); !ok {
						misses <- "first Store"
					}
				}()
				go func() {
					defer wg.Done()
					<-start
					switch mode {
					case "Store":
						m.Store(7, "second")
					case "LoadOrStore":
						m.LoadOrStore(7, "second")
					case "LoadOrStoreLazy":
						m.LoadOrStoreLazy(7, func() interface{} { return "second" })
					}
					if _, ok := m.Load(7); !ok {
						misses <- mode
					}
				}()
				close(start)
				wg.Wait()
				close(misses)
				for operation := range misses {
					t.Fatalf("round=%d: %s returned, subsequent Load missing; no Delete occurred", i, operation)
				}
			}
			t.Logf("%d bounded two-writer histories had no observed missing key; this is not a proof for all schedules", rounds)
		})
	}
}
