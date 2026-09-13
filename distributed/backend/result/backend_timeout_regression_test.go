package result

import (
	"context"
	"errors"
	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
	"reflect"
	"sync"
	"testing"
	"time"
)

type deadlineReadBackend struct {
	pendingResultBackend
	blockID string
	entered chan context.Context
	release chan struct{}
}

func (b *deadlineReadBackend) GetStatus(id string) (*task.Status, error) {
	<-b.release
	return &task.Status{TaskID: id, Status: task.StateSuccess}, nil
}
func (b *deadlineReadBackend) GetStatusContext(ctx context.Context, id string) (*task.Status, error) {
	if id != b.blockID {
		return &task.Status{TaskID: id, Status: task.StateSuccess}, nil
	}
	b.entered <- ctx
	<-ctx.Done()
	return &task.Status{TaskID: id, Status: task.StateSuccess}, nil // Even a late success must lose to the deadline.
}

type legacyGateBackend struct {
	once sync.Once
	pendingResultBackend
	entered chan struct{}
	release chan struct{}
}

func (b *legacyGateBackend) GetStatus(id string) (*task.Status, error) {
	b.once.Do(func() { close(b.entered) })
	<-b.release
	return &task.Status{TaskID: id, Status: task.StateSuccess}, nil
}

func timeoutGetters(b backend.Backend) map[string]func(time.Duration, time.Duration) ([]reflect.Value, error) {
	return map[string]func(time.Duration, time.Duration) ([]reflect.Value, error){
		"async": NewAsyncResult(&task.Signature{ID: "async"}, b).GetWithTimeout,
		"chain": NewChainAsyncResult([]*task.Signature{{ID: "first"}, {ID: "last"}}, b).GetWithTimeout,
		"group": NewGroupCallbackAsyncResult([]*task.Signature{{ID: "member"}}, &task.Signature{ID: "callback"}, b).GetWithTimeout,
	}
}

func TestTimeoutIncludesContextBackendRead(t *testing.T) {
	for _, tc := range []struct{ name, id string }{{"async", "async"}, {"chain", "first"}, {"chain", "last"}, {"group", "member"}, {"group", "callback"}} {
		name := tc.name
		t.Run(name+"/"+tc.id, func(t *testing.T) {
			b := &deadlineReadBackend{blockID: tc.id, entered: make(chan context.Context, 1), release: make(chan struct{})}
			done := make(chan error, 1)
			go func() { _, err := timeoutGetters(b)[name](20*time.Millisecond, time.Hour); done <- err }()
			select {
			case err := <-done:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("read timeout = %v", err)
				}
			case <-time.After(time.Second):
				close(b.release)
				<-done
				t.Fatal("backend read did not honor deadline")
			}
			select {
			case ctx := <-b.entered:
				if ctx.Err() != context.DeadlineExceeded {
					t.Fatalf("read context %v", ctx.Err())
				}
			default:
				t.Fatal("context-aware read not used")
			}
		})
	}
}

func TestLegacyTimeoutDiscardsLateSuccess(t *testing.T) {
	for _, name := range []string{"async", "chain", "group"} {
		t.Run(name, func(t *testing.T) {
			b := &legacyGateBackend{entered: make(chan struct{}), release: make(chan struct{})}
			done := make(chan error, 1)
			// Legacy reads cannot be canceled. A controlled gate verifies that a
			// result returned after the budget is discarded, without a goroutine wrapper.
			go func() { _, err := timeoutGetters(b)[name](20*time.Millisecond, time.Hour); done <- err }()
			<-b.entered
			timer := time.NewTimer(40 * time.Millisecond)
			defer timer.Stop()
			<-timer.C
			close(b.release)
			if err := <-done; !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("late success: %v", err)
			}
		})
	}
}
