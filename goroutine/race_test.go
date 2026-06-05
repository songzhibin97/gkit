package goroutine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/timeout"
)

// TestAddTask_ShutdownRace stresses the AddTask vs Shutdown path that used to
// race (close(g.task) concurrent with send) and the wg.Add (AddTask->_go pool
// growth) vs wg.Wait (Shutdown) race the growMu guard fixes. A single
// create/shutdown cycle rarely hits the window, so amplify across many fresh
// pools. Run under -race; reverting the growMu guard reproduces the race here.
func TestAddTask_ShutdownRace(t *testing.T) {
	for k := 0; k < 100; k++ {
		// Start small (SetIdle 1) with room to grow (SetMax 16) and submit
		// briefly-busy tasks, so the pool keeps calling _go (g.wait.Add) to
		// grow while Shutdown calls g.wait.Wait — the window the growMu guard
		// closes. Instant tasks never fill the channel, so the pool never grows
		// and the race stays hidden.
		g := NewGoroutine(context.Background(), SetMax(16), SetIdle(1))
		var wg sync.WaitGroup
		for i := 0; i < 16; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					g.AddTask(func() { time.Sleep(20 * time.Microsecond) })
				}
			}()
		}
		time.Sleep(100 * time.Microsecond) // let the pool start growing first
		_ = g.Shutdown()
		wg.Wait()
	}
}

// TestDelegate_PropagatesTimeoutToInner verifies C6: f receives a context that
// already carries the requested timeout. f returns immediately based on whether
// its ctx has a deadline, so its result wins the outer select deterministically
// (no race with the 20ms outer deadline). The previous assertion was satisfied
// by Delegate's own outer-timeout path and passed even when f got a
// deadline-less context.
func TestDelegate_PropagatesTimeoutToInner(t *testing.T) {
	sentinel := errors.New("inner saw deadline")
	err := Delegate(context.Background(), 20*time.Millisecond, func(ctx context.Context) error {
		if _, ok := ctx.Deadline(); ok {
			return sentinel
		}
		return errors.New("inner context had no deadline")
	})
	if err != sentinel {
		t.Fatalf("f did not receive a deadline-bearing context: %v", err)
	}
}

func TestDelegate_NoTimeoutLetsFFinish(t *testing.T) {
	want := errors.New("done")
	err := Delegate(context.Background(), 0, func(ctx context.Context) error {
		return want
	})
	if !errors.Is(err, want) {
		t.Fatalf("err = %v, want %v", err, want)
	}
}

func TestDelegate_ShrinkReducesDeadline(t *testing.T) {
	// Quick sanity that timeout.Shrink really clamps the deadline below the
	// parent's. If the parent has no deadline, the shrunk ctx must have one.
	parent, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	_, shrunk, c2 := timeout.Shrink(parent, 5*time.Millisecond)
	defer c2()
	d, ok := shrunk.Deadline()
	if !ok || time.Until(d) > 50*time.Millisecond {
		t.Fatalf("shrunk deadline = %v, ok=%v", d, ok)
	}
}
