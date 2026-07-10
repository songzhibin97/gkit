package egroup

import (
	"context"
	"errors"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type issue80AsyncGroup struct {
	mu               sync.Mutex
	closed           bool
	addCalls         int
	addAfterShutdown int
	shutdownCalls    int
}

func (g *issue80AsyncGroup) ChangeMax(int64) {}

func (g *issue80AsyncGroup) AddTask(f func()) bool {
	g.mu.Lock()
	if g.closed {
		g.addAfterShutdown++
		g.mu.Unlock()
		return false
	}
	g.addCalls++
	g.mu.Unlock()

	go f()
	return true
}

func (g *issue80AsyncGroup) AddTaskN(ctx context.Context, f func()) bool {
	select {
	case <-ctx.Done():
		return false
	default:
		return g.AddTask(f)
	}
}

func (g *issue80AsyncGroup) Shutdown() error {
	g.mu.Lock()
	g.closed = true
	g.shutdownCalls++
	g.mu.Unlock()
	return nil
}

func (*issue80AsyncGroup) Trick() string { return "" }

func (g *issue80AsyncGroup) counts() (addCalls, addAfterShutdown, shutdownCalls int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.addCalls, g.addAfterShutdown, g.shutdownCalls
}

type issue80RejectGroup struct {
	addCalls atomic.Int32
}

func (*issue80RejectGroup) ChangeMax(int64) {}

func (g *issue80RejectGroup) AddTask(func()) bool {
	g.addCalls.Add(1)
	return false
}

func (g *issue80RejectGroup) AddTaskN(context.Context, func()) bool {
	g.addCalls.Add(1)
	return false
}

func (*issue80RejectGroup) Shutdown() error { return nil }
func (*issue80RejectGroup) Trick() string   { return "" }

type issue80CancelRejectGroup struct {
	ctx     context.Context
	started chan struct{}
	once    sync.Once
}

func (*issue80CancelRejectGroup) ChangeMax(int64) {}

func (g *issue80CancelRejectGroup) AddTask(func()) bool {
	g.once.Do(func() { close(g.started) })
	<-g.ctx.Done()
	return false
}

func (g *issue80CancelRejectGroup) AddTaskN(context.Context, func()) bool {
	return g.AddTask(nil)
}

func (*issue80CancelRejectGroup) Shutdown() error { return nil }
func (*issue80CancelRejectGroup) Trick() string   { return "" }

type issue80ContextAwareBlockingGroup struct {
	started       chan struct{}
	legacyRelease chan struct{}
	startOnce     sync.Once
	releaseOnce   sync.Once
}

func (*issue80ContextAwareBlockingGroup) ChangeMax(int64) {}

func (g *issue80ContextAwareBlockingGroup) markStarted() {
	g.startOnce.Do(func() { close(g.started) })
}

func (g *issue80ContextAwareBlockingGroup) AddTask(func()) bool {
	g.markStarted()
	<-g.legacyRelease
	return false
}

func (g *issue80ContextAwareBlockingGroup) AddTaskN(ctx context.Context, _ func()) bool {
	g.markStarted()
	<-ctx.Done()
	return false
}

func (*issue80ContextAwareBlockingGroup) Shutdown() error { return nil }
func (*issue80ContextAwareBlockingGroup) Trick() string   { return "" }

func (g *issue80ContextAwareBlockingGroup) releaseLegacyAdd() {
	g.releaseOnce.Do(func() { close(g.legacyRelease) })
}

func newIssue80LifeAdmin(stop time.Duration) *LifeAdmin {
	ctx, cancel := context.WithCancel(context.Background())
	g := WithContextGroup(ctx, &issue80AsyncGroup{})
	return &LifeAdmin{
		opts: &config{
			startTimeout: -1,
			stopTimeout:  stop,
		},
		shutdown: cancel,
		g:        g,
	}
}

type issue80ShutdownObservation struct {
	err      error
	deadline time.Time
	hasLimit bool
}

func TestLifeAdminShutdownUsesFreshBoundedContext(t *testing.T) {
	t.Run("waits for callback completion", func(t *testing.T) {
		const stop = 2 * time.Second
		admin := newIssue80LifeAdmin(stop)

		startReady := make(chan struct{})
		startStopped := make(chan struct{})
		shutdownEntered := make(chan issue80ShutdownObservation, 1)
		shutdownFinished := make(chan struct{})
		releaseShutdown := make(chan struct{})
		var releaseOnce sync.Once
		release := func() { releaseOnce.Do(func() { close(releaseShutdown) }) }
		defer release()

		admin.Add(Member{
			Start: func(ctx context.Context) error {
				close(startReady)
				<-ctx.Done()
				close(startStopped)
				return nil
			},
			Shutdown: func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				shutdownEntered <- issue80ShutdownObservation{
					err:      ctx.Err(),
					deadline: deadline,
					hasLimit: ok,
				}
				<-releaseShutdown
				close(shutdownFinished)
				return nil
			},
		})

		startDone := make(chan error, 1)
		go func() { startDone <- admin.Start() }()
		waitIssue80Signal(t, startReady, "member start")

		admin.Shutdown()
		observation := waitIssue80Value(t, shutdownEntered, "member shutdown")
		assertIssue80FreshBoundedContext(t, observation, stop)
		waitIssue80Signal(t, startStopped, "member start cancellation")

		select {
		case err := <-startDone:
			t.Fatalf("Start returned before shutdown callback completed: %v", err)
		case <-time.After(50 * time.Millisecond):
		}

		release()
		select {
		case err := <-startDone:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("Start error = %v, want nil or context cancellation", err)
			}
		case <-time.After(time.Second):
			t.Fatal("Start did not return after shutdown callback completed")
		}
		waitIssue80Signal(t, shutdownFinished, "shutdown callback completion")
	})

	t.Run("waits until stop timeout", func(t *testing.T) {
		const stop = 120 * time.Millisecond
		admin := newIssue80LifeAdmin(stop)

		startReady := make(chan struct{})
		shutdownEntered := make(chan issue80ShutdownObservation, 1)
		shutdownTimedOut := make(chan struct{})
		admin.Add(Member{
			Start: func(ctx context.Context) error {
				close(startReady)
				<-ctx.Done()
				return nil
			},
			Shutdown: func(ctx context.Context) error {
				deadline, ok := ctx.Deadline()
				shutdownEntered <- issue80ShutdownObservation{
					err:      ctx.Err(),
					deadline: deadline,
					hasLimit: ok,
				}
				<-ctx.Done()
				close(shutdownTimedOut)
				return ctx.Err()
			},
		})

		startDone := make(chan error, 1)
		go func() { startDone <- admin.Start() }()
		waitIssue80Signal(t, startReady, "member start")

		startedAt := time.Now()
		admin.Shutdown()
		observation := waitIssue80Value(t, shutdownEntered, "member shutdown")
		assertIssue80FreshBoundedContext(t, observation, stop)

		select {
		case <-startDone:
			elapsed := time.Since(startedAt)
			if elapsed < stop/2 {
				t.Fatalf("Start returned after %v, before stop timeout %v", elapsed, stop)
			}
		case <-time.After(time.Second):
			t.Fatal("Start did not return after stop timeout")
		}
		waitIssue80Signal(t, shutdownTimedOut, "shutdown context timeout")
	})
}

func TestGroupShutdownCancelsBeforeWaiting(t *testing.T) {
	backend := &issue80AsyncGroup{}
	g := WithContextGroup(context.Background(), backend)
	taskStarted := make(chan struct{})
	taskCanceled := make(chan struct{})
	g.Go(func() error {
		close(taskStarted)
		<-g.ctx.Done()
		close(taskCanceled)
		return nil
	})
	waitIssue80Signal(t, taskStarted, "group task")

	shutdownDone := make(chan error, 1)
	go func() { shutdownDone <- g.Shutdown() }()

	select {
	case <-taskCanceled:
	case <-time.After(500 * time.Millisecond):
		g.cancel()
		<-shutdownDone
		t.Fatal("Shutdown waited for task before canceling group context")
	}

	select {
	case err := <-shutdownDone:
		if err != nil {
			t.Fatalf("Shutdown error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Shutdown did not return after the canceled task exited")
	}

	if err := g.Shutdown(); err != nil {
		t.Fatalf("repeated Shutdown error = %v, want nil", err)
	}
}

func TestGroupGoShutdownRegistrationIsSerialized(t *testing.T) {
	t.Run("shutdown cancels a context-aware blocked registration", func(t *testing.T) {
		backend := &issue80ContextAwareBlockingGroup{
			started:       make(chan struct{}),
			legacyRelease: make(chan struct{}),
		}
		defer backend.releaseLegacyAdd()
		g := WithContextGroup(context.Background(), backend)

		var ran atomic.Bool
		goDone := make(chan struct{})
		go func() {
			g.Go(func() error {
				ran.Store(true)
				return nil
			})
			close(goDone)
		}()
		waitIssue80Signal(t, backend.started, "context-aware task registration")

		shutdownDone := make(chan error, 1)
		go func() { shutdownDone <- g.Shutdown() }()

		select {
		case err := <-shutdownDone:
			if err != nil {
				t.Fatalf("Shutdown error = %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			// Unblock the legacy AddTask path so a broken implementation does not
			// leak test goroutines after the assertion.
			backend.releaseLegacyAdd()
			<-goDone
			<-shutdownDone
			t.Fatal("Shutdown did not cancel a blocked task registration")
		}
		waitIssue80Signal(t, goDone, "context-aware Go rejection")
		if ran.Load() {
			t.Fatal("task ran after its registration was canceled")
		}
	})

	t.Run("registered submission is released by shutdown cancellation", func(t *testing.T) {
		backend := &issue80CancelRejectGroup{started: make(chan struct{})}
		g := WithContextGroup(context.Background(), backend)
		backend.ctx = g.ctx

		var ran atomic.Bool
		goDone := make(chan struct{})
		go func() {
			g.Go(func() error {
				ran.Store(true)
				return nil
			})
			close(goDone)
		}()
		waitIssue80Signal(t, backend.started, "task registration")

		shutdownDone := make(chan error, 1)
		go func() { shutdownDone <- g.Shutdown() }()

		select {
		case err := <-shutdownDone:
			if err != nil {
				t.Fatalf("Shutdown error = %v", err)
			}
		case <-time.After(500 * time.Millisecond):
			g.cancel()
			<-shutdownDone
			t.Fatal("Shutdown did not cancel a concurrently registered submission")
		}
		waitIssue80Signal(t, goDone, "Go rejection")
		if ran.Load() {
			t.Fatal("task ran after backend rejected the registered submission")
		}
	})

	t.Run("stress concurrent registration and shutdown", func(t *testing.T) {
		const rounds = 500
		for round := 0; round < rounds; round++ {
			backend := &issue80AsyncGroup{}
			g := WithContextGroup(context.Background(), backend)
			start := make(chan struct{})
			panicValue := make(chan interface{}, 5)
			var callers sync.WaitGroup
			callers.Add(5)

			for i := 0; i < 4; i++ {
				go func() {
					defer callers.Done()
					defer func() {
						if recovered := recover(); recovered != nil {
							panicValue <- recovered
						}
					}()
					<-start
					runtime.Gosched()
					g.Go(func() error { return nil })
				}()
			}
			go func() {
				defer callers.Done()
				defer func() {
					if recovered := recover(); recovered != nil {
						panicValue <- recovered
					}
				}()
				<-start
				_ = g.Shutdown()
			}()

			close(start)
			callers.Wait()
			close(panicValue)
			for recovered := range panicValue {
				t.Fatalf("round %d panicked during Go/Shutdown: %v", round, recovered)
			}

			_, addAfterShutdown, _ := backend.counts()
			if addAfterShutdown != 0 {
				t.Fatalf("round %d submitted %d tasks after backend shutdown", round, addAfterShutdown)
			}
		}
	})
}

func TestGroupRejectedTaskReturnsErrorFromWait(t *testing.T) {
	backend := &issue80RejectGroup{}
	g := WithContextGroup(context.Background(), backend)
	var ran atomic.Bool

	g.Go(func() error {
		ran.Store(true)
		return nil
	})

	if ran.Load() {
		t.Fatal("task ran even though the backend rejected it")
	}
	if err := g.Wait(); !errors.Is(err, ErrGroupClosed) {
		t.Fatalf("Wait error = %v, want %v", err, ErrGroupClosed)
	}
	if got := backend.addCalls.Load(); got != 1 {
		t.Fatalf("backend AddTask calls = %d, want 1", got)
	}
	select {
	case <-g.ctx.Done():
	default:
		t.Fatal("backend rejection did not cancel group context")
	}
}

func assertIssue80FreshBoundedContext(t *testing.T, got issue80ShutdownObservation, stop time.Duration) {
	t.Helper()
	if got.err != nil {
		t.Fatalf("shutdown context was already canceled: %v", got.err)
	}
	if !got.hasLimit {
		t.Fatal("shutdown context has no deadline")
	}
	remaining := time.Until(got.deadline)
	if remaining <= 0 || remaining > stop+50*time.Millisecond {
		t.Fatalf("shutdown context deadline remaining = %v, want (0, %v]", remaining, stop+50*time.Millisecond)
	}
}

func waitIssue80Signal(t *testing.T, ch <-chan struct{}, operation string) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", operation)
	}
}

func waitIssue80Value[T any](t *testing.T, ch <-chan T, operation string) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", operation)
		var zero T
		return zero
	}
}
