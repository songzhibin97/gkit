package result

import (
	"errors"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
)

var errBackendRead = errors.New("backend read failed")

type failingReadBackend struct {
	getStatusCalls atomic.Int32
}

func (*failingReadBackend) GroupTakeOver(string, string, ...string) error { return nil }
func (*failingReadBackend) GroupCompleted(string) (bool, error)           { return false, nil }
func (*failingReadBackend) GroupTaskStatus(string) ([]*task.Status, error) {
	return nil, nil
}
func (*failingReadBackend) TriggerCompleted(string) (bool, error) { return false, nil }
func (*failingReadBackend) SetStatePending(*task.Signature) error { return nil }
func (*failingReadBackend) SetStateReceived(*task.Signature) error {
	return nil
}
func (*failingReadBackend) SetStateStarted(*task.Signature) error { return nil }
func (*failingReadBackend) SetStateRetry(*task.Signature) error   { return nil }
func (*failingReadBackend) SetStateSuccess(*task.Signature, []*task.Result) error {
	return nil
}
func (*failingReadBackend) SetStateFailure(*task.Signature, string) error { return nil }
func (b *failingReadBackend) GetStatus(string) (*task.Status, error) {
	b.getStatusCalls.Add(1)
	return nil, errBackendRead
}
func (*failingReadBackend) ResetTask(...string) error  { return nil }
func (*failingReadBackend) ResetGroup(...string) error { return nil }
func (*failingReadBackend) SetResultExpire(int64)      {}

func newFailingAsyncResult() (*AsyncResult, *failingReadBackend, *task.Status) {
	const taskID = "task-read-error"
	b := &failingReadBackend{}
	asyncResult := NewAsyncResult(&task.Signature{ID: taskID}, b)
	cached := &task.Status{TaskID: taskID, Status: task.StatePending}
	asyncResult.state = cached
	return asyncResult, b, cached
}

func TestAsyncResultGetStateWithErrorPreservesCachedState(t *testing.T) {
	asyncResult, backend, cached := newFailingAsyncResult()

	state, err := asyncResult.GetStateWithError()
	if state != cached {
		t.Fatalf("GetStateWithError state = %p, want cached %p", state, cached)
	}
	assertWrappedBackendReadError(t, err)
	if got := backend.getStatusCalls.Load(); got != 1 {
		t.Fatalf("GetStateWithError backend calls = %d, want 1", got)
	}

	// The compatibility API keeps its original signature, deliberately drops
	// the backend error, and still returns the last cached state.
	if state := asyncResult.GetState(); state != cached {
		t.Fatalf("GetState state = %p, want cached %p", state, cached)
	}
	if got := backend.getStatusCalls.Load(); got != 2 {
		t.Fatalf("GetState backend calls = %d, want 2", got)
	}
}

func TestAsyncResultBackendReadErrorsReturnImmediately(t *testing.T) {
	tests := []struct {
		name string
		call func(*AsyncResult) ([]reflect.Value, error)
	}{
		{
			name: "Monitor",
			call: func(result *AsyncResult) ([]reflect.Value, error) {
				return result.Monitor()
			},
		},
		{
			name: "Get",
			call: func(result *AsyncResult) ([]reflect.Value, error) {
				return result.Get(time.Hour)
			},
		},
		{
			name: "GetWithTimeout",
			call: func(result *AsyncResult) ([]reflect.Value, error) {
				return result.GetWithTimeout(time.Hour, time.Hour)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asyncResult, backend, _ := newFailingAsyncResult()
			done := make(chan error, 1)
			go func() {
				_, err := tt.call(asyncResult)
				done <- err
			}()

			select {
			case err := <-done:
				assertWrappedBackendReadError(t, err)
			case <-time.After(250 * time.Millisecond):
				t.Fatal("backend read error was treated as pending and kept polling")
			}
			if got := backend.getStatusCalls.Load(); got != 1 {
				t.Fatalf("backend calls = %d, want exactly 1", got)
			}
		})
	}
}

func assertWrappedBackendReadError(t *testing.T, err error) {
	t.Helper()
	if !errors.Is(err, errBackendRead) {
		t.Fatalf("error = %v, want wrapped backend sentinel", err)
	}
	if !strings.Contains(err.Error(), "task-read-error") {
		t.Fatalf("error lacks task ID context: %v", err)
	}
}
