package egroup

import (
	"context"
	"errors"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/goroutine"
)

type lifecycleObservedPool struct {
	goroutine.GGroup
	registered chan struct{}
}

func (p *lifecycleObservedPool) AddTaskN(ctx context.Context, task func()) bool {
	ok := p.GGroup.AddTaskN(ctx, task)
	if ok {
		p.registered <- struct{}{}
	}
	return ok
}

// Regression for #165, item 02-03: lifecycle waiters must not consume the
// finite worker capacity needed to start members and register signal handling.
func TestLifeAdminFinitePoolStartsAllMembers(t *testing.T) {
	for _, test := range []struct {
		name     string
		workers  int64
		members  int
		shutdown bool
		signals  bool
	}{
		{name: "single_worker_start_and_shutdown", workers: 1, members: 1, shutdown: true},
		{name: "single_worker_start_only", workers: 1, members: 1},
		{name: "single_worker_signal_only", workers: 1, signals: true},
		{name: "single_worker_signal_watcher", workers: 1, members: 1, shutdown: true, signals: true},
		{name: "multiple_workers_control", workers: 8, members: 2, shutdown: true, signals: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := &lifecycleObservedPool{
				GGroup:     goroutine.NewGoroutine(context.Background(), goroutine.SetMax(test.workers), goroutine.SetIdle(test.workers)),
				registered: make(chan struct{}, 8),
			}
			group := WithContextGroup(context.Background(), backend)
			admin := NewLifeAdmin(SetGroup(group), SetSignal(nil), SetStopTimeout(time.Second))
			if test.signals {
				SetSignal(func(*LifeAdmin, os.Signal) {}, syscall.SIGUSR1)(admin.opts)
			}
			started := make(chan struct{}, test.members)
			stopped := make(chan issue80ShutdownObservation, test.members)
			startCompletions := make(chan struct{}, test.members)
			wantErr := errors.New("synthetic member shutdown error")
			for i := 0; i < test.members; i++ {
				member := Member{Start: func(ctx context.Context) error {
					started <- struct{}{}
					<-ctx.Done()
					startCompletions <- struct{}{}
					return nil
				}}
				if test.shutdown {
					member.Shutdown = func(ctx context.Context) error {
						deadline, bounded := ctx.Deadline()
						stopped <- issue80ShutdownObservation{err: ctx.Err(), deadline: deadline, hasLimit: bounded}
						return wantErr
					}
				}
				admin.Add(member)
			}
			result := make(chan error, 1)
			finished := make(chan struct{})
			go func() {
				defer close(finished)
				result <- admin.Start()
			}()
			t.Cleanup(func() {
				admin.Shutdown()
				select {
				case <-finished:
					if err := group.Shutdown(); err != nil {
						t.Error(err)
					}
				case <-time.After(2 * time.Second):
					t.Error("LifeAdmin did not finish during cleanup")
				}
			})
			for i := 0; i < test.members; i++ {
				waitIssue80Signal(t, started, "member start with finite worker pool")
			}
			registrations := test.members
			if test.shutdown {
				registrations += test.members
			}
			if test.signals {
				registrations++
			}
			for i := 0; i < registrations; i++ {
				waitIssue80Signal(t, backend.registered, "lifecycle registration including signal watcher")
			}
			if workers := lifeAdminPoolWorkerCount(t, group); workers != int(test.workers) {
				t.Fatalf("pool worker count = %d, want %d", workers, test.workers)
			}
			admin.Shutdown()
			err := waitIssue80Value(t, result, "LifeAdmin.Start result")
			if test.shutdown {
				if !errors.Is(err, wantErr) {
					t.Fatalf("Start returned %v, want shutdown error %v", err, wantErr)
				}
				for i := 0; i < test.members; i++ {
					assertIssue80FreshBoundedContext(t, waitIssue80Value(t, stopped, "member Shutdown callback"), time.Second)
				}
			} else if err != nil {
				t.Fatalf("Start-only cancellation returned %v, want nil", err)
			}
			// Delegate may return on cancellation before the member observes it.
			// Join the actual callbacks separately without sleeping.
			for i := 0; i < test.members; i++ {
				waitIssue80Signal(t, startCompletions, "actual Start callback exit")
			}
		})
	}
}

func TestLifeAdminRejectedControllerReleasesWaitCount(t *testing.T) {
	backend := &issue80RejectGroup{}
	group := WithContextGroup(context.Background(), backend)
	admin := NewLifeAdmin(SetGroup(group), SetSignal(nil))
	called := make(chan struct{}, 1)
	admin.Add(Member{Shutdown: func(context.Context) error { called <- struct{}{}; return nil }})
	result := make(chan error, 1)
	go func() { result <- admin.Start() }()
	if err := waitIssue80Value(t, result, "rejected controller completion"); !errors.Is(err, ErrGroupClosed) {
		t.Fatalf("Start after rejected controller = %v, want %v", err, ErrGroupClosed)
	}
	if got := backend.addCalls.Load(); got != 1 {
		t.Fatalf("submission attempts = %d, want 1", got)
	}
	select {
	case <-called:
		t.Fatal("rejected Shutdown callback ran")
	default:
	}
	if err := group.Shutdown(); err != nil {
		t.Fatal(err)
	}
}

func TestLifeAdminSignalHandlerPanicIsContained(t *testing.T) {
	backend := &lifecycleObservedPool{
		GGroup:     goroutine.NewGoroutine(context.Background(), goroutine.SetMax(1), goroutine.SetIdle(1)),
		registered: make(chan struct{}, 2),
	}
	group := WithContextGroup(context.Background(), backend)
	wantErr := errors.New("synthetic signal handler panic")
	admin := NewLifeAdmin(SetGroup(group), SetSignal(func(*LifeAdmin, os.Signal) { panic(wantErr) }, syscall.SIGUSR1))
	stopped := make(chan struct{})
	admin.Add(Member{Shutdown: func(context.Context) error { close(stopped); return nil }})
	result, finished := make(chan error, 1), make(chan struct{})
	go func() { defer close(finished); result <- admin.Start() }()
	t.Cleanup(func() {
		admin.Shutdown()
		waitIssue80Signal(t, finished, "signal panic cleanup")
		if err := group.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	// The second accepted submission follows signal.Notify, so sending the
	// signal to this test process cannot precede handler registration.
	for i := 0; i < 2; i++ {
		waitIssue80Signal(t, backend.registered, "signal handler registration")
	}
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatal(err)
	}
	if err := process.Signal(syscall.SIGUSR1); err != nil {
		t.Fatal(err)
	}
	if err := waitIssue80Value(t, result, "signal panic result"); !errors.Is(err, wantErr) {
		t.Fatalf("Start after handler panic = %v, want %v", err, wantErr)
	}
	waitIssue80Signal(t, stopped, "Shutdown after signal panic")
}

func TestLifeAdminStartCallbacksKeepPoolConcurrencyLimit(t *testing.T) {
	backend := &lifecycleObservedPool{
		GGroup:     goroutine.NewGoroutine(context.Background(), goroutine.SetMax(1), goroutine.SetIdle(1)),
		registered: make(chan struct{}, 8),
	}
	group := WithContextGroup(context.Background(), backend)
	admin := NewLifeAdmin(SetGroup(group), SetSignal(func(*LifeAdmin, os.Signal) {}, syscall.SIGUSR1))
	firstStarted, secondStarted, releaseFirst := make(chan struct{}), make(chan struct{}), make(chan struct{})
	admin.Add(Member{
		Start:    func(context.Context) error { close(firstStarted); <-releaseFirst; return nil },
		Shutdown: func(context.Context) error { return nil },
	})
	admin.Add(Member{
		Start:    func(ctx context.Context) error { close(secondStarted); <-ctx.Done(); return nil },
		Shutdown: func(context.Context) error { return nil },
	})
	result, finished := make(chan error, 1), make(chan struct{})
	go func() { defer close(finished); result <- admin.Start() }()
	t.Cleanup(func() {
		select {
		case <-releaseFirst:
		default:
			close(releaseFirst)
		}
		admin.Shutdown()
		waitIssue80Signal(t, finished, "concurrency-test cleanup")
		if err := group.Shutdown(); err != nil {
			t.Error(err)
		}
	})
	waitIssue80Signal(t, firstStarted, "first Start")
	// Two shutdown controllers and the signal watcher must be registered
	// before the first Start occupies the sole pool worker.
	for i := 0; i < 4; i++ {
		waitIssue80Signal(t, backend.registered, "controller/first Start registration")
	}
	select {
	case <-secondStarted:
		t.Fatal("second Start bypassed the configured single-worker limit")
	default:
	}
	probeRan := make(chan struct{})
	probeCtx, cancelProbe := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancelProbe()
	if backend.GGroup.AddTaskN(probeCtx, func() { close(probeRan) }) {
		t.Fatal("ordinary pool probe was accepted while first Start should occupy the sole worker")
	}
	if !errors.Is(probeCtx.Err(), context.DeadlineExceeded) {
		t.Fatalf("pool probe was rejected without reaching its wait deadline: %v", probeCtx.Err())
	}
	close(releaseFirst)
	waitIssue80Signal(t, secondStarted, "second Start after first completes")
	waitIssue80Signal(t, backend.registered, "second Start registration")
	select {
	case <-probeRan:
		t.Fatal("expired pool probe executed after capacity became available")
	default:
	}
	admin.Shutdown()
	if err := waitIssue80Value(t, result, "concurrency-test Start completion"); err != nil {
		t.Fatalf("Start returned %v, want nil", err)
	}
}
