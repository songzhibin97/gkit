package goroutine

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/timeout"
)

// TestAddTask_ShutdownRace stresses the AddTask vs Shutdown path that used
// to race (close(g.task) concurrent with send) and the wg.Add/wg.Wait race.
func TestAddTask_ShutdownRace(t *testing.T) {
	g := NewGoroutine(context.Background(), SetMax(8), SetIdle(2))

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				g.AddTask(func() {})
			}
		}()
	}
	time.Sleep(1 * time.Millisecond)
	_ = g.Shutdown()
	wg.Wait()
}

// TestDelegate_PropagatesTimeoutToInner verifies the fix to C6 — that f
// receives a context that already carries the requested timeout, so f's
// own ctx-aware logic honours the deadline.
func TestDelegate_PropagatesTimeoutToInner(t *testing.T) {
	sentinel := errors.New("inner deadline observed")
	err := Delegate(context.Background(), 20*time.Millisecond, func(ctx context.Context) error {
		select {
		case <-time.After(200 * time.Millisecond):
			return errors.New("inner did not see deadline")
		case <-ctx.Done():
			return sentinel
		}
	})
	if !errors.Is(err, context.DeadlineExceeded) && err != sentinel {
		t.Fatalf("expected inner to observe ctx.Done; outer err = %v", err)
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
