package goroutine

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestInvalidCheckTimeKeepsPoolUsable(t *testing.T) {
	tests := []struct {
		name      string
		checkTime time.Duration
	}{
		{name: "zero", checkTime: 0},
		{name: "negative", checkTime: -time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := NewGoroutine(
				context.Background(),
				SetMax(1),
				SetIdle(1),
				SetCheckTime(tt.checkTime),
			)
			defer func() { _ = group.Shutdown() }()
			impl := group.(*Goroutine)
			if got := impl.checkTime; got != 10*time.Minute {
				t.Fatalf("checkTime = %v, want %v", got, 10*time.Minute)
			}

			executed := make(chan struct{})
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			if !group.AddTaskN(ctx, func() { close(executed) }) {
				t.Fatal("AddTaskN rejected a task after invalid checkTime was normalized")
			}
			select {
			case <-executed:
			case <-ctx.Done():
				t.Fatalf("task did not execute: %v", ctx.Err())
			}
			if got := atomic.LoadInt64(&impl.n); got != 1 {
				t.Fatalf("worker count = %d, want 1", got)
			}
		})
	}
}

func TestInvalidMaxIsNormalizedToOne(t *testing.T) {
	tests := []struct {
		name string
		max  int64
	}{
		{name: "zero", max: 0},
		{name: "negative", max: -1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			group := NewGoroutine(context.Background(), SetMax(tt.max), SetIdle(0))
			defer func() { _ = group.Shutdown() }()
			impl := group.(*Goroutine)
			if got := atomic.LoadInt64(&impl.max); got != 1 {
				t.Fatalf("max = %d, want 1", got)
			}
			group.ChangeMax(tt.max)
			if got := atomic.LoadInt64(&impl.max); got != 1 {
				t.Fatalf("max after ChangeMax = %d, want 1", got)
			}

			executed := make(chan struct{})
			ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
			defer cancel()
			if !group.AddTaskN(ctx, func() { close(executed) }) {
				t.Fatal("AddTaskN rejected a task after invalid max was normalized")
			}
			select {
			case <-executed:
			case <-ctx.Done():
				t.Fatalf("task did not execute: %v", ctx.Err())
			}
		})
	}
}

func TestChangeMaxRestartsEmptyPoolForBlockedAddTask(t *testing.T) {
	group := NewGoroutine(context.Background(), SetMax(1), SetIdle(0))
	defer func() { _ = group.Shutdown() }()
	impl := group.(*Goroutine)

	// Model the legacy state created by ChangeMax(0): no worker is allowed to
	// receive from the unbuffered task channel, so AddTask is already blocked.
	atomic.StoreInt64(&impl.max, 0)
	addTaskBlocked := make(chan struct{})
	impl.ctx = &doneSignalContext{Context: impl.ctx, called: addTaskBlocked}
	taskStarted := make(chan struct{})
	releaseTask := make(chan struct{})
	var releaseOnce sync.Once
	release := func() { releaseOnce.Do(func() { close(releaseTask) }) }
	defer release()
	executed := make(chan struct{})
	accepted := make(chan bool, 1)
	go func() {
		accepted <- group.AddTask(func() {
			close(taskStarted)
			<-releaseTask
			close(executed)
		})
	}()

	select {
	case <-addTaskBlocked:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("AddTask did not reach its blocking submission select")
	}

	group.ChangeMax(1)
	if got := atomic.LoadInt64(&impl.n); got != 1 {
		t.Fatalf("worker count after ChangeMax = %d, want exactly 1", got)
	}
	select {
	case <-taskStarted:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("restored worker did not start the blocked task")
	}
	release()
	select {
	case ok := <-accepted:
		if !ok {
			t.Fatal("blocked AddTask was rejected after ChangeMax restored capacity")
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("ChangeMax did not start a worker for the blocked AddTask")
	}
	select {
	case <-executed:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("accepted task did not execute")
	}
	if got := atomic.LoadInt64(&impl.n); got != 1 {
		t.Fatalf("worker count after task execution = %d, want exactly 1", got)
	}
}

func TestValidConfigurationAndShutdownRemainStable(t *testing.T) {
	group := NewGoroutine(
		context.Background(),
		SetMax(2),
		SetIdle(1),
		SetCheckTime(time.Hour),
	)
	impl := group.(*Goroutine)
	executed := make(chan struct{})
	if !group.AddTask(func() { close(executed) }) {
		t.Fatal("AddTask rejected a valid task")
	}
	select {
	case <-executed:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("valid task did not execute")
	}
	group.ChangeMax(2)
	if err := group.Shutdown(); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	waitForWorkerCount(t, impl, 0)
	group.ChangeMax(2)
	if got := atomic.LoadInt64(&impl.n); got != 0 {
		t.Fatalf("ChangeMax started %d worker(s) after Shutdown", got)
	}
	if group.AddTask(func() {}) {
		t.Fatal("AddTask accepted work after Shutdown")
	}
	if err := group.Shutdown(); !errors.Is(err, ErrRepeatClose) {
		t.Fatalf("second Shutdown error = %v, want %v", err, ErrRepeatClose)
	}
}

func TestChangeMaxShutdownRace(t *testing.T) {
	for round := 0; round < 100; round++ {
		group := NewGoroutine(context.Background(), SetMax(1), SetIdle(0))
		impl := group.(*Goroutine)
		var wg sync.WaitGroup
		for worker := 0; worker < 4; worker++ {
			wg.Add(1)
			go func(worker int) {
				defer wg.Done()
				for i := 0; i < 20; i++ {
					if (worker+i)%2 == 0 {
						group.ChangeMax(0)
					} else {
						group.ChangeMax(2)
					}
				}
			}(worker)
		}
		if err := group.Shutdown(); err != nil {
			t.Fatalf("round %d Shutdown: %v", round, err)
		}
		wg.Wait()
		waitForWorkerCount(t, impl, 0)
	}
}

func waitForWorkerCount(t *testing.T, group *Goroutine, want int64) {
	t.Helper()
	deadline := time.Now().Add(250 * time.Millisecond)
	for {
		if got := atomic.LoadInt64(&group.n); got == want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("worker count = %d, want %d", atomic.LoadInt64(&group.n), want)
		}
		time.Sleep(time.Millisecond)
	}
}

type doneSignalContext struct {
	context.Context
	called chan struct{}
	once   sync.Once
}

func (c *doneSignalContext) Done() <-chan struct{} {
	c.once.Do(func() { close(c.called) })
	return c.Context.Done()
}
