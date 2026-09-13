package watching

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func TestGCNotificationAndStopAreSynchronized(t *testing.T) {
	// Reproduce the state installed by Start before the GC consumer runs.
	// Each object is invoked once, with no existing finalizer, as it would
	// be when the runtime calls it. Keep it alive until finalizer cleanup.
	makeFixture := func(t *testing.T) (*Watching, *gcHeapFinalizer, chan struct{}) {
		t.Helper()
		w := NewWatching(WithLoggerLevel(-1))
		ch := make(chan struct{}, 1)
		w.gcEventsCh = ch
		w.rptEventsCh = make(chan rptEvent, 32)
		atomic.StoreInt64(&w.stopped, 0)
		gc := &gcHeapFinalizer{w: w}
		t.Cleanup(func() {
			runtime.SetFinalizer(gc, nil)
			runtime.KeepAlive(gc)
		})
		return w, gc, ch
	}

	t.Run("accepted_event_survives_close", func(t *testing.T) {
		w, gc, ch := makeFixture(t)
		finalizerCallback(gc)
		w.Stop()
		if _, ok := <-ch; !ok {
			t.Fatal("GC notification disappeared before the closed queue was drained")
		}
		if _, ok := <-ch; ok {
			t.Fatal("Stop did not close the GC queue after its accepted event")
		}
	})

	t.Run("full_queue_remains_nonblocking", func(t *testing.T) {
		w, gc, ch := makeFixture(t)
		ch <- struct{}{}
		finalizerCallback(gc)
		w.Stop()
		if _, ok := <-ch; !ok {
			t.Fatal("full queue lost its pending GC notification")
		}
		if _, ok := <-ch; ok {
			t.Fatal("full queue retained an unexpected second notification")
		}
	})

	// The race detector must also observe field access and send/close;
	// successful functional assertions alone do not prove synchronization.
	t.Run("concurrent", func(t *testing.T) {
		for round := 0; round < 256; round++ {
			w, gc, _ := makeFixture(t)
			start := make(chan struct{})
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				<-start
				finalizerCallback(gc)
			}()
			go func() {
				defer wg.Done()
				<-start
				w.Stop()
			}()
			close(start)
			wg.Wait()
			if w.gcEventsCh != nil || atomic.LoadInt64(&w.stopped) != 1 {
				t.Fatal("Stop did not complete GC channel teardown")
			}
		}
	})
}
