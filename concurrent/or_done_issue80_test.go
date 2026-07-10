package concurrent

import (
	"testing"
	"time"
)

func requireClosedSignal(t *testing.T, signal <-chan interface{}) {
	t.Helper()
	select {
	case value, ok := <-signal:
		if ok {
			t.Fatalf("completion signal forwarded payload %v", value)
		}
		if value != nil {
			t.Fatalf("closed completion signal value = %v, want nil", value)
		}
	case <-time.After(time.Second):
		t.Fatal("completion signal did not close")
	}
}

func requireOpenSignal(t *testing.T, signal <-chan interface{}) {
	t.Helper()
	select {
	case value, ok := <-signal:
		t.Fatalf("completion signal fired unexpectedly: value=%v ok=%v", value, ok)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestOrDoneNoInputsIsClosedSignal(t *testing.T) {
	requireClosedSignal(t, OrDone())
}

func TestOrDoneSingleInputIsSignalOnly(t *testing.T) {
	t.Run("payload", func(t *testing.T) {
		source := make(chan interface{}, 1)
		source <- "payload"
		requireClosedSignal(t, OrDone(source))
	})

	t.Run("closed", func(t *testing.T) {
		source := make(chan interface{})
		close(source)
		requireClosedSignal(t, OrDone(source))
	})

	t.Run("nil", func(t *testing.T) {
		requireOpenSignal(t, OrDone(nil))
	})
}

func TestOrDoneMultipleInputsAndNilSemantics(t *testing.T) {
	t.Run("first payload closes signal", func(t *testing.T) {
		ready := make(chan interface{}, 1)
		ready <- "payload"
		other := make(chan interface{})
		requireClosedSignal(t, OrDone(nil, other, ready))
	})

	t.Run("first close closes signal", func(t *testing.T) {
		closed := make(chan interface{})
		close(closed)
		other := make(chan interface{})
		requireClosedSignal(t, OrDone(other, nil, closed))
	})

	t.Run("all nil remains open", func(t *testing.T) {
		requireOpenSignal(t, OrDone(nil, nil))
	})
}
