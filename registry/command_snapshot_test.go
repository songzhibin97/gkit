package registry

import (
	"context"
	"reflect"
	"runtime"
	"testing"
	"time"
)

func TestServiceCommandsSnapshotIDsBeforeQueueing(t *testing.T) {
	for _, remove := range []bool{false, true} {
		name := "add"
		if remove {
			name = "remove"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// Start the real sentinel only after enqueue/reuse. This deterministically
			// exercises the permitted delay between acceptance and execution.
			r := &RockSteadierSubset{clients: []int{1}, hasClient: map[int]int{1: 0}, hasService: make(map[int][2]int), ctx: ctx, cancel: cancel, command: make(chan command, 10)}
			r.matrixServices.Store([][]*int{{}})
			r.addService([]int{999})
			if remove {
				r.addService([]int{10, 20})
			}
			update := r.AddService
			if remove {
				update = r.RemoveService
			}
			ids := []int{10}
			if err := update(ctx, ids); err != nil {
				t.Fatal(err)
			}
			ids[0] = 20
			if err := update(ctx, ids); err != nil {
				t.Fatal(err)
			}
			ids[0] = 999
			if len(r.command) != 2 {
				t.Fatalf("queued commands=%d want=2", len(r.command))
			}
			// The marker is consumed after both accepted commands by the FIFO sentinel.
			if err := r.AddService(ctx, []int{30}); err != nil {
				t.Fatal(err)
			}
			r.sentinel()
			defer r.Close()
			deadline := time.Now().Add(time.Second)
			for {
				got := issue83Sorted(r.GetServices(1))
				seen := false
				for _, id := range got {
					if id == 30 {
						seen = true
					}
				}
				if seen {
					want := []int{10, 20, 30, 999}
					if remove {
						want = []int{30, 999}
					}
					if !reflect.DeepEqual(got, want) {
						t.Fatalf("after FIFO marker services=%v want=%v", got, want)
					}
					break
				}
				if time.Now().After(deadline) {
					t.Fatalf("FIFO marker not observed: %v", got)
				}
				runtime.Gosched()
			}
		})
	}
}
