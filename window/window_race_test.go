package window

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// TestAddIndex_ShutdownRace exercises the Shutdown / AddIndex TOCTOU that
// used to panic with `send on closed channel`. After the fix Shutdown no
// longer closes communication and AddIndex drops on ctx.Done.
func TestAddIndex_ShutdownRace(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	w := &Window{
		conf: conf{
			size:     8,
			interval: 10 * time.Millisecond,
			ctx:      ctx,
		},
		cancel:        cancel,
		communication: make(chan Index),
		buffer:        make([]atomic.Value, 8),
	}
	for i := range w.buffer {
		w.buffer[i].Store(map[string]uint{})
	}
	go w.sentinel()

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				w.AddIndex("k", 1)
			}
		}()
	}
	time.Sleep(2 * time.Millisecond)
	w.Shutdown()
	wg.Wait()
}
