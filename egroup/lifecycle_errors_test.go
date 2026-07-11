package egroup

import (
	"context"
	"errors"
	"testing"
	"time"
)

type lifecycleControlledGroup struct {
	registered chan struct{}
	completed  chan struct{}
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

func (*lifecycleControlledGroup) Shutdown() error { return nil }
func (*lifecycleControlledGroup) Trick() string   { return "" }

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
