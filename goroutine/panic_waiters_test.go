package goroutine

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/log"
)

type panicRecordLogger struct {
	records chan []interface{}
}

func (l *panicRecordLogger) Log(level log.Lever, values ...interface{}) error {
	if level == log.LevelError {
		l.records <- append([]interface{}(nil), values...)
	}
	return nil
}

// Regression for #165, item 02-02: a recovered task panic must not strand a
// submission which already passed the pool-growth check at maximum capacity.
func TestTaskPanicKeepsBlockedSubmitterProgressing(t *testing.T) {
	for _, api := range []string{"AddTask", "AddTaskN"} {
		t.Run(api, func(t *testing.T) {
			logger := &panicRecordLogger{records: make(chan []interface{}, 8)}
			group := NewGoroutine(context.Background(), SetMax(1), SetIdle(1), SetLogger(logger))
			g := group.(*Goroutine)
			t.Cleanup(func() {
				if err := group.Shutdown(); err != nil {
					t.Error(err)
				}
			})
			for round := 0; round < 2; round++ {
				started, release := make(chan struct{}), make(chan struct{})
				var once sync.Once
				unblock := func() { once.Do(func() { close(release) }) }
				t.Cleanup(unblock)
				if !group.AddTask(func() {
					close(started)
					<-release
					panic("synthetic task panic")
				}) {
					t.Fatal("initial task was rejected")
				}
				awaitPanicTaskSignal(t, started, "initial task did not start")
				// The worker is inside the gated task, so it cannot read g.ctx
				// while this observer is installed. Releasing the task publishes
				// the observer before the worker next evaluates its select.
				blocked := make(chan struct{})
				g.ctx = &doneSignalContext{Context: g.ctx, called: blocked}
				executed := make(chan struct{})
				accepted := make(chan bool, 1)
				go func() {
					task := func() {
						close(executed)
						panic("synthetic task panic")
					}
					if api == "AddTask" {
						accepted <- group.AddTask(task)
					} else {
						accepted <- group.AddTaskN(context.Background(), task)
					}
				}()
				awaitPanicTaskSignal(t, blocked, "submitter did not enter blocking select")
				if got := atomic.LoadInt64(&g.n); got != 1 {
					t.Fatalf("workers with blocked submission = %d, want exactly 1", got)
				}
				unblock()
				select {
				case ok := <-accepted:
					if !ok {
						t.Fatal("pre-existing submission was rejected after panic")
					}
				case <-time.After(2 * time.Second):
					t.Fatalf("pre-existing %s stranded after panic; workers=%d", api, atomic.LoadInt64(&g.n))
				}
				awaitPanicTaskSignal(t, executed, "waiting task did not execute")
				for i := 0; i < 2; i++ {
					select {
					case record := <-logger.records:
						if len(record) != 4 || record[0] != "panic err:" || record[1] != "synthetic task panic" || record[2] != "panic stack:" {
							t.Fatalf("panic log = %#v", record)
						}
						stack, ok := record[3].(string)
						if !ok || !strings.Contains(stack, "panic_waiters_test.go") {
							t.Fatalf("panic stack does not identify task: %v", record[3])
						}
					case <-time.After(2 * time.Second):
						t.Fatal("task panic was not recorded")
					}
				}
				if got := atomic.LoadInt64(&g.n); got != 1 {
					t.Fatalf("workers after repeated panic = %d, want exactly 1", got)
				}
			}
			completed := make(chan struct{})
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			if !group.AddTaskN(ctx, func() { close(completed) }) {
				t.Fatal("normal task after repeated panics was rejected")
			}
			awaitPanicTaskSignal(t, completed, "normal task did not finish")
		})
	}
}

func TestTaskPanicDuringShutdownDoesNotRestartWorker(t *testing.T) {
	logger := &panicRecordLogger{records: make(chan []interface{}, 1)}
	group := NewGoroutine(context.Background(), SetMax(1), SetIdle(1), SetLogger(logger))
	g := group.(*Goroutine)
	started, release := make(chan struct{}), make(chan struct{})
	if !group.AddTask(func() {
		close(started)
		<-release
		panic("synthetic shutdown panic")
	}) {
		t.Fatal("task rejected")
	}
	awaitPanicTaskSignal(t, started, "task did not start")
	done := make(chan error, 1)
	go func() { done <- group.Shutdown() }()
	awaitPanicTaskSignal(t, g.ctx.Done(), "Shutdown did not cancel worker context")
	close(release)
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Shutdown after panic: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Shutdown did not join worker after panic")
	}
	waitForWorkerCount(t, g, 0)
	if group.AddTask(func() { t.Error("task ran after Shutdown") }) || group.AddTaskN(context.Background(), func() { t.Error("context task ran after Shutdown") }) {
		t.Fatal("submission accepted after Shutdown")
	}
	if got := atomic.LoadInt64(&g.n); got != 0 {
		t.Fatalf("workers after rejected submissions = %d, want 0", got)
	}
}

func awaitPanicTaskSignal(t *testing.T, signal <-chan struct{}, failure string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(2 * time.Second):
		t.Fatal(failure)
	}
}
