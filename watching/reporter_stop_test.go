package watching

import (
	"bytes"
	"sync"
	"sync/atomic"
	"testing"
)

type stopTestReporter struct{}

func (*stopTestReporter) Report(string, []byte, string, string) error { return nil }

// Reproduce the channel state installed by Start before its consumer is
// scheduled. The real producer and public Stop then race without a network
// reporter or runtime profile capture. Run with -race to detect send/close
// and channel-field races as well as the functional assertions below.
func TestProfileSubmissionAndStopAreSynchronized(t *testing.T) {
	for round := 0; round < 128; round++ {
		w := NewWatching(WithProfileReporter(&stopTestReporter{}), WithLoggerLevel(-1))
		events := make(chan rptEvent, 1)
		w.rptEventsCh = events
		atomic.StoreInt64(&w.stopped, 0)
		first := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			w.reportProfile("thread", []byte("profile"), "threshold", "first")
			close(first)
			for i := 0; i < 128; i++ {
				w.reportProfile("thread", []byte("later"), "threshold", "later")
			}
		}()
		go func() {
			defer wg.Done()
			<-first
			w.Stop()
		}()
		wg.Wait()
		if atomic.LoadInt64(&w.stopped) != 1 || w.rptEventsCh != nil {
			t.Fatal("Stop did not clear the reporter channel")
		}
		// Closing must retain the event accepted before Stop. The full queue
		// must neither block the later submissions nor replace that event.
		event, ok := <-events
		if !ok || event.PType != "thread" || event.Reason != "threshold" || event.EventID != "first" || !bytes.Equal(event.Buf, []byte("profile")) {
			t.Fatalf("accepted event changed or disappeared: %+v, ok=%v", event, ok)
		}
		w.reportProfile("thread", []byte("after stop"), "threshold", "after stop")
		if _, ok := <-events; ok {
			t.Fatal("closed reporter queue contains an unexpected event")
		}
	}
}
