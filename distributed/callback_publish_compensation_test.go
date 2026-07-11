package distributed

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend"
	backendredis "github.com/songzhibin97/gkit/distributed/backend/backend_redis"
	"github.com/songzhibin97/gkit/distributed/task"
)

const wantTaskPublicationFailureMessage = "task publication outcome unknown"

type callbackPublishController struct {
	publish func(context.Context, *task.Signature) error
}

func (*callbackPublishController) RegisterTask(...string)     {}
func (*callbackPublishController) IsRegisterTask(string) bool { return true }
func (*callbackPublishController) StartConsuming(int, task.Processor) (bool, error) {
	return false, nil
}
func (*callbackPublishController) StopConsuming() {}
func (c *callbackPublishController) Publish(ctx context.Context, signature *task.Signature) error {
	if c.publish != nil {
		return c.publish(ctx, signature)
	}
	return nil
}
func (*callbackPublishController) GetPendingTasks(string) ([]*task.Signature, error) { return nil, nil }
func (*callbackPublishController) GetDelayedTasks() ([]*task.Signature, error)       { return nil, nil }
func (*callbackPublishController) SetConsumingQueue(string)                          {}
func (*callbackPublishController) SetDelayedQueue(string)                            {}

type callbackCompensationFailBackend struct {
	backend.Backend
	err error
}

func (b *callbackCompensationFailBackend) SetStateFailure(*task.Signature, string) error {
	return b.err
}

func newCallbackPublishTestServer(t *testing.T, publish func(context.Context, *task.Signature) error) (*Server, backend.Backend) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	b := backendredis.NewBackendRedis(client, -1)
	return &Server{
		config:     &Config{ConsumeQueue: "callback-test"},
		backend:    b,
		controller: &callbackPublishController{publish: publish},
	}, b
}

func assertCallbackTaskState(t *testing.T, b backend.Backend, taskID string, wantState task.State, wantError string) {
	t.Helper()
	status, err := b.GetStatus(taskID)
	if err != nil {
		t.Fatalf("GetStatus(%q): %v", taskID, err)
	}
	if status.Status != wantState {
		t.Fatalf("status %q = %s, want %s", taskID, status.Status, wantState)
	}
	if status.Error != wantError {
		t.Fatalf("status %q error = %q, want %q", taskID, status.Error, wantError)
	}
}

func assertPendingAtPublish(t *testing.T, b backend.Backend, signature *task.Signature) {
	t.Helper()
	assertCallbackTaskState(t, b, signature.ID, task.StatePending, "")
}

func TestSendTaskPublishFailureConvergesPendingState(t *testing.T) {
	publishErr := errors.New("broker publish failed")
	var b backend.Backend
	server, b := newCallbackPublishTestServer(t, func(_ context.Context, signature *task.Signature) error {
		assertPendingAtPublish(t, b, signature)
		return publishErr
	})
	signature := task.NewSignature("callback-failed", "callback")

	asyncResult, err := server.SendTaskWithContext(context.Background(), signature)
	if asyncResult != nil {
		t.Fatalf("async result = %#v, want nil on publication failure", asyncResult)
	}
	if !errors.Is(err, publishErr) {
		t.Fatalf("error = %v, want wrapped publication error", err)
	}
	if !strings.Contains(err.Error(), "publish task callback-failed") {
		t.Fatalf("error = %v, want task publication context", err)
	}
	assertCallbackTaskState(t, b, signature.ID, task.StateFailure, wantTaskPublicationFailureMessage)
}

func TestSendTaskPublishCompensationPreservesBothErrors(t *testing.T) {
	publishErr := errors.New("broker publish failed")
	compensationErr := errors.New("persist failure state failed")
	var baseBackend backend.Backend
	server, baseBackend := newCallbackPublishTestServer(t, func(_ context.Context, signature *task.Signature) error {
		assertPendingAtPublish(t, baseBackend, signature)
		return publishErr
	})
	server.backend = &callbackCompensationFailBackend{Backend: baseBackend, err: compensationErr}

	asyncResult, err := server.SendTaskWithContext(context.Background(), task.NewSignature("callback-compensation-failed", "callback"))
	if asyncResult != nil {
		t.Fatalf("async result = %#v, want nil on publication failure", asyncResult)
	}
	if !errors.Is(err, publishErr) || !errors.Is(err, compensationErr) {
		t.Fatalf("error = %v, want publication and compensation errors", err)
	}
	if !strings.Contains(err.Error(), "publish task callback-compensation-failed") ||
		!strings.Contains(err.Error(), "converge task callback-compensation-failed") {
		t.Fatalf("error = %v, want publish and convergence context", err)
	}
}

func TestSendTaskRetryAfterPublishFailure(t *testing.T) {
	publishErr := errors.New("broker publish failed")
	attempts := 0
	var b backend.Backend
	server, b := newCallbackPublishTestServer(t, func(_ context.Context, signature *task.Signature) error {
		assertPendingAtPublish(t, b, signature)
		attempts++
		if attempts == 1 {
			return publishErr
		}
		return nil
	})
	signature := task.NewSignature("callback-retry", "callback")

	if asyncResult, err := server.SendTask(signature); asyncResult != nil || !errors.Is(err, publishErr) {
		t.Fatalf("first send = (%#v, %v), want nil and publication error", asyncResult, err)
	}
	assertCallbackTaskState(t, b, signature.ID, task.StateFailure, wantTaskPublicationFailureMessage)

	asyncResult, err := server.SendTask(signature)
	if err != nil || asyncResult == nil {
		t.Fatalf("retry send = (%#v, %v), want async result and nil error", asyncResult, err)
	}
	assertCallbackTaskState(t, b, signature.ID, task.StatePending, "")
}

func TestSendTaskAcceptedButPublishReturnedErrorUsesExistingLastWriteSemantics(t *testing.T) {
	publishErr := errors.New("publish acknowledgement lost")
	var b backend.Backend
	server, b := newCallbackPublishTestServer(t, func(_ context.Context, signature *task.Signature) error {
		assertPendingAtPublish(t, b, signature)
		if err := b.SetStateSuccess(signature, nil); err != nil {
			t.Fatalf("simulate worker success: %v", err)
		}
		assertCallbackTaskState(t, b, signature.ID, task.StateSuccess, "")
		return publishErr
	})
	signature := task.NewSignature("accepted-before-error", "callback")

	asyncResult, err := server.SendTask(signature)
	if asyncResult != nil || !errors.Is(err, publishErr) {
		t.Fatalf("send = (%#v, %v), want nil and ambiguous publication error", asyncResult, err)
	}
	assertCallbackTaskState(t, b, signature.ID, task.StateFailure, wantTaskPublicationFailureMessage)
}

func TestOrdinaryCallbacksUsePublishCompensation(t *testing.T) {
	tests := []struct {
		name        string
		publishErr  error
		wantState   task.State
		wantMessage string
		dispatch    func(*Worker, *task.Signature) error
	}{
		{
			name:      "successful success callback remains pending",
			wantState: task.StatePending,
			dispatch: func(worker *Worker, parent *task.Signature) error {
				return worker.handlerSucceeded(parent, nil)
			},
		},
		{
			name:      "successful error callback remains pending",
			wantState: task.StatePending,
			dispatch: func(worker *Worker, parent *task.Signature) error {
				worker.errorHandler = func(error) {}
				return worker.handlerFailed(parent, errors.New("parent task failed"))
			},
		},
		{
			name:        "failed success callback becomes failed",
			publishErr:  errors.New("success callback publish failed"),
			wantState:   task.StateFailure,
			wantMessage: wantTaskPublicationFailureMessage,
			dispatch: func(worker *Worker, parent *task.Signature) error {
				return worker.handlerSucceeded(parent, nil)
			},
		},
		{
			name:        "failed error callback becomes failed",
			publishErr:  errors.New("error callback publish failed"),
			wantState:   task.StateFailure,
			wantMessage: wantTaskPublicationFailureMessage,
			dispatch: func(worker *Worker, parent *task.Signature) error {
				worker.errorHandler = func(error) {}
				return worker.handlerFailed(parent, errors.New("parent task failed"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b backend.Backend
			server, b := newCallbackPublishTestServer(t, func(_ context.Context, signature *task.Signature) error {
				assertPendingAtPublish(t, b, signature)
				return tt.publishErr
			})
			callback := task.NewSignature("ordinary-callback", "callback")
			parent := task.NewSignature("parent", "parent")
			parent.CallbackOnSuccess = []*task.Signature{callback}
			parent.CallbackOnError = []*task.Signature{callback}
			worker := &Worker{bindService: server}

			if err := tt.dispatch(worker, parent); err != nil {
				t.Fatalf("dispatch ordinary callback: %v", err)
			}
			assertCallbackTaskState(t, b, callback.ID, tt.wantState, tt.wantMessage)
		})
	}
}
