package egroup

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/goroutine"
)

type lifecycleControlledGroup struct {
	registered chan struct{}
	completed  chan struct{}
	shutdowns  atomic.Int32
	closed     atomic.Bool
}

func newLifecycleControlledGroup() *lifecycleControlledGroup {
	return &lifecycleControlledGroup{
		registered: make(chan struct{}, 4),
		completed:  make(chan struct{}, 4),
	}
}

func (*lifecycleControlledGroup) ChangeMax(int64) {}

func (g *lifecycleControlledGroup) AddTask(f func()) bool {
	return g.AddTaskN(context.Background(), f)
}

func (g *lifecycleControlledGroup) AddTaskN(ctx context.Context, f func()) bool {
	if g.closed.Load() {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	default:
	}
	g.registered <- struct{}{}
	go func() {
		f()
		g.completed <- struct{}{}
	}()
	return true
}

func (g *lifecycleControlledGroup) Shutdown() error {
	g.closed.Store(true)
	g.shutdowns.Add(1)
	return nil
}
func (*lifecycleControlledGroup) Trick() string { return "" }

func TestLifeAdminSetGroupShutdownCancelsSuppliedGroup(t *testing.T) {
	backend := newLifecycleControlledGroup()
	group := WithContextGroup(context.Background(), backend)
	admin := NewLifeAdmin(SetGroup(group), SetSignal(nil))

	memberStarted := make(chan struct{})
	memberStopped := make(chan struct{})
	shutdownCalled := make(chan struct{})
	admin.Add(Member{
		Start: func(ctx context.Context) error {
			close(memberStarted)
			<-ctx.Done()
			close(memberStopped)
			return nil
		},
		Shutdown: func(context.Context) error {
			close(shutdownCalled)
			return nil
		},
	})

	startDone := make(chan error, 1)
	go func() { startDone <- admin.Start() }()
	waitIssue80Signal(t, backend.registered, "shutdown task registration")
	waitIssue80Signal(t, backend.registered, "start task registration")
	waitIssue80Signal(t, memberStarted, "member start")

	admin.Shutdown()
	stoppedByAdmin := false
	select {
	case <-memberStopped:
		stoppedByAdmin = true
	case <-time.After(100 * time.Millisecond):
		// Clean up the old implementation, whose LifeAdmin cancel is unrelated
		// to the supplied group's context.
		group.cancel()
		waitIssue80Signal(t, memberStopped, "member cleanup")
	}

	err := waitIssue80Value(t, startDone, "LifeAdmin.Start completion")
	waitIssue80Signal(t, shutdownCalled, "member shutdown")
	if !stoppedByAdmin {
		t.Fatal("LifeAdmin.Shutdown did not cancel the group supplied by SetGroup")
	}
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("LifeAdmin.Start error = %v, want nil or context cancellation", err)
	}
}

func TestLifeAdminSignalWatcherDoesNotMaskShutdownError(t *testing.T) {
	backend := newLifecycleControlledGroup()
	group := WithContextGroup(context.Background(), backend)
	admin := NewLifeAdmin(SetGroup(group), SetStopTimeout(time.Second))

	want := errors.New("shutdown failed")
	shutdownEntered := make(chan struct{})
	releaseShutdown := make(chan struct{})
	admin.Add(Member{Shutdown: func(context.Context) error {
		close(shutdownEntered)
		<-releaseShutdown
		return want
	}})

	startDone := make(chan error, 1)
	go func() { startDone <- admin.Start() }()
	waitIssue80Signal(t, backend.registered, "shutdown task registration")
	waitIssue80Signal(t, backend.registered, "signal task registration")

	group.cancel()
	waitIssue80Signal(t, shutdownEntered, "member shutdown start")
	// The shutdown callback is still blocked, so this completion can only be
	// the signal watcher. Waiting for it removes scheduler timing from the test.
	waitIssue80Signal(t, backend.completed, "signal watcher completion")
	close(releaseShutdown)

	if err := waitIssue80Value(t, startDone, "LifeAdmin.Start completion"); !errors.Is(err, want) {
		t.Fatalf("LifeAdmin.Start error = %v, want shutdown error %v", err, want)
	}
}

func TestLifeAdminStartCancellationDoesNotMaskShutdownError(t *testing.T) {
	backend := newLifecycleControlledGroup()
	group := WithContextGroup(context.Background(), backend)
	admin := NewLifeAdmin(SetGroup(group), SetSignal(nil), SetStopTimeout(time.Second))

	want := errors.New("shutdown failed")
	shutdownEntered := make(chan struct{})
	releaseShutdown := make(chan struct{})
	admin.Add(Member{
		Start: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
		Shutdown: func(context.Context) error {
			close(shutdownEntered)
			<-releaseShutdown
			return want
		},
	})

	startDone := make(chan error, 1)
	go func() { startDone <- admin.Start() }()
	waitIssue80Signal(t, backend.registered, "shutdown task registration")
	waitIssue80Signal(t, backend.registered, "start task registration")

	admin.Shutdown()
	waitIssue80Signal(t, shutdownEntered, "member shutdown start")
	// The shutdown callback is still blocked, so the first completion is the
	// member Start wrapper reacting to normal group cancellation.
	waitIssue80Signal(t, backend.completed, "member start wrapper completion")
	close(releaseShutdown)

	if err := waitIssue80Value(t, startDone, "LifeAdmin.Start completion"); !errors.Is(err, want) {
		t.Fatalf("LifeAdmin.Start error = %v, want shutdown error %v", err, want)
	}
}

func TestLifeAdminPreservesMemberStartError(t *testing.T) {
	backend := newLifecycleControlledGroup()
	group := WithContextGroup(context.Background(), backend)
	admin := NewLifeAdmin(SetGroup(group), SetSignal(nil))

	want := fmt.Errorf("member start failed: %w", context.Canceled)
	releaseStart := make(chan struct{})
	admin.Add(Member{
		Start: func(context.Context) error {
			<-releaseStart
			return want
		},
		Shutdown: func(context.Context) error { return nil },
	})

	startDone := make(chan error, 1)
	go func() { startDone <- admin.Start() }()
	waitIssue80Signal(t, backend.registered, "shutdown task registration")
	waitIssue80Signal(t, backend.registered, "start task registration")
	close(releaseStart)

	if err := waitIssue80Value(t, startDone, "LifeAdmin.Start completion"); !errors.Is(err, want) {
		t.Fatalf("LifeAdmin.Start error = %v, want member start error %v", err, want)
	}
}

func TestLifeAdminSetGroupStopsOwnedPool(t *testing.T) {
	group := WithContext(
		context.Background(),
		goroutine.SetMax(1),
		goroutine.SetIdle(1),
	)
	defer func() { _ = group.goroutine.Shutdown() }()
	admin := NewLifeAdmin(SetGroup(group), SetSignal(nil))

	memberStarted := make(chan struct{})
	admin.Add(Member{Start: func(ctx context.Context) error {
		close(memberStarted)
		<-ctx.Done()
		return nil
	}})
	startDone := make(chan error, 1)
	go func() { startDone <- admin.Start() }()
	waitIssue80Signal(t, memberStarted, "member start")
	if got := lifeAdminPoolWorkerCount(t, group); got != 1 {
		t.Fatalf("pool workers before shutdown = %d, want 1", got)
	}

	admin.Shutdown()
	if err := waitIssue80Value(t, startDone, "LifeAdmin.Start completion"); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("LifeAdmin.Start error = %v, want nil or context cancellation", err)
	}
	if !waitLifeAdminPoolWorkerCount(t, group, 0) {
		t.Fatalf("pool workers after LifeAdmin.Shutdown = %d, want 0", lifeAdminPoolWorkerCount(t, group))
	}
}

func TestLifeAdminSetGroupDoesNotShutdownExternalPool(t *testing.T) {
	backend := newLifecycleControlledGroup()
	group := WithContextGroup(context.Background(), backend)
	admin := NewLifeAdmin(SetGroup(group), SetSignal(nil))

	memberStarted := make(chan struct{})
	admin.Add(Member{Start: func(ctx context.Context) error {
		close(memberStarted)
		<-ctx.Done()
		return nil
	}})
	startDone := make(chan error, 1)
	go func() { startDone <- admin.Start() }()
	waitIssue80Signal(t, backend.registered, "start task registration")
	waitIssue80Signal(t, memberStarted, "member start")

	admin.Shutdown()
	if err := waitIssue80Value(t, startDone, "LifeAdmin.Start completion"); err != nil && !errors.Is(err, context.Canceled) {
		t.Fatalf("LifeAdmin.Start error = %v, want nil or context cancellation", err)
	}
	if got := backend.shutdowns.Load(); got != 0 {
		t.Fatalf("external pool Shutdown calls = %d, want 0", got)
	}
	externalTaskRan := make(chan struct{})
	if ok := backend.AddTask(func() { close(externalTaskRan) }); !ok {
		t.Fatal("external pool rejected a task after LifeAdmin.Shutdown")
	}
	waitIssue80Signal(t, externalTaskRan, "external pool task")
}

func waitLifeAdminPoolWorkerCount(t *testing.T, group *Group, want int) bool {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		if lifeAdminPoolWorkerCount(t, group) == want {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return lifeAdminPoolWorkerCount(t, group) == want
}

func lifeAdminPoolWorkerCount(t *testing.T, group *Group) int {
	t.Helper()
	var max, idle, workers, taskLen int
	if _, err := fmt.Sscanf(
		group.goroutine.Trick(),
		"max: %d idle: %d now goroutine: %d task len: %d",
		&max,
		&idle,
		&workers,
		&taskLen,
	); err != nil {
		t.Fatalf("parse pool state: %v", err)
	}
	return workers
}
