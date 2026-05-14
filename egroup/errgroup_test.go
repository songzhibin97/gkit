package egroup

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/goroutine"
)

// TestGroupGoAfterShutdownDoesNotDeadlock verifies that when the
// underlying goroutine pool refuses a task (e.g. because Shutdown ran),
// Group.Go releases the wg counter it pre-incremented so that subsequent
// Wait calls don't block forever.
func TestGroupGoAfterShutdownDoesNotDeadlock(t *testing.T) {
	pool := goroutine.NewGoroutine(context.Background())
	g := WithContextGroup(context.Background(), pool)

	if err := pool.Shutdown(); err != nil {
		t.Fatalf("pool shutdown: %v", err)
	}

	// AddTask now returns false; Go must compensate the wg.Add(1).
	g.Go(func() error { return nil })
	g.Go(func() error { return nil })

	done := make(chan struct{})
	go func() {
		g.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Wait blocked after AddTask refused — wg counter leaked")
	}
}

func TestGroupGoNormalRunPropagatesError(t *testing.T) {
	g := WithContext(context.Background())
	defer func() { _ = g.Shutdown() }()

	want := errors.New("boom")
	var ran sync.WaitGroup
	ran.Add(1)
	g.Go(func() error {
		defer ran.Done()
		return want
	})
	ran.Wait()
	// Give the Once + cancel time to settle.
	time.Sleep(50 * time.Millisecond)
	if err := g.Wait(); !errors.Is(err, want) {
		t.Fatalf("Wait err = %v, want %v", err, want)
	}
}
