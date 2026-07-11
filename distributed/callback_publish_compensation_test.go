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

type publicationAttemptBackendForTest interface {
	SetStatePendingAttempt(*task.Signature, string) error
	FailPendingAttempt(*task.Signature, string, string) (bool, error)
}

func (b *callbackCompensationFailBackend) SetStateFailure(signature *task.Signature, reason string) error {
	if reason == taskPublicationFailureMessage {
		return b.err
	}
	return b.Backend.SetStateFailure(signature, reason)
}

func (b *callbackCompensationFailBackend) SetStatePendingAttempt(signature *task.Signature, attemptID string) error {
	return b.Backend.(publicationAttemptBackendForTest).SetStatePendingAttempt(signature, attemptID)
}

func (b *callbackCompensationFailBackend) FailPendingAttempt(*task.Signature, string, string) (bool, error) {
	return false, b.err
}

type blockingCallbackCompensationBackend struct {
	backend.Backend
	started chan struct{}
	release chan struct{}
}

func (b *blockingCallbackCompensationBackend) SetStateFailure(signature *task.Signature, reason string) error {
	close(b.started)
	<-b.release
	return b.Backend.SetStateFailure(signature, reason)
}

func (b *blockingCallbackCompensationBackend) SetStatePendingAttempt(signature *task.Signature, attemptID string) error {
	return b.Backend.(publicationAttemptBackendForTest).SetStatePendingAttempt(signature, attemptID)
}

func (b *blockingCallbackCompensationBackend) FailPendingAttempt(signature *task.Signature, attemptID, reason string) (bool, error) {
	close(b.started)
	<-b.release
	return b.Backend.(publicationAttemptBackendForTest).FailPendingAttempt(signature, attemptID, reason)
}

type unsupportedCompensationBackend struct {
	groupTestBackend
	failureWrites int
}

func (b *unsupportedCompensationBackend) SetStateFailure(*task.Signature, string) error {
	b.failureWrites++
	return nil
}

type attemptTrackingBackend struct {
	groupTestBackend
	pendingAttempts int
}

func (b *attemptTrackingBackend) SetStatePendingAttempt(*task.Signature, string) error {
	b.pendingAttempts++
	return nil
}

func (*attemptTrackingBackend) FailPendingAttempt(*task.Signature, string, string) (bool, error) {
	return false, nil
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

func TestSendTaskSuccessfulPublishLeavesOwnedPendingState(t *testing.T) {
	var b backend.Backend
	server, b := newCallbackPublishTestServer(t, func(_ context.Context, signature *task.Signature) error {
		assertPendingAtPublish(t, b, signature)
		return nil
	})
	signature := task.NewSignature("callback-published", "callback")

	asyncResult, err := server.SendTaskWithContext(context.Background(), signature)
	if err != nil || asyncResult == nil {
		t.Fatalf("send = (%#v, %v), want async result and nil error", asyncResult, err)
	}
	assertCallbackTaskState(t, b, signature.ID, task.StatePending, "")
}

func TestSendTaskPublishFailureConvergesOwnedPendingAttempt(t *testing.T) {
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

func TestSendTaskPublishFailurePreservesAdvancedState(t *testing.T) {
	publishErr := errors.New("publish acknowledgement lost")
	tests := []struct {
		name       string
		wantState  task.State
		wantError  string
		transition func(backend.Backend, *task.Signature) error
	}{
		{name: "received", wantState: task.StateReceived, transition: func(b backend.Backend, signature *task.Signature) error { return b.SetStateReceived(signature) }},
		{name: "started", wantState: task.StateStarted, transition: func(b backend.Backend, signature *task.Signature) error { return b.SetStateStarted(signature) }},
		{name: "retry", wantState: task.StateRetry, transition: func(b backend.Backend, signature *task.Signature) error { return b.SetStateRetry(signature) }},
		{name: "success", wantState: task.StateSuccess, transition: func(b backend.Backend, signature *task.Signature) error { return b.SetStateSuccess(signature, nil) }},
		{name: "failure", wantState: task.StateFailure, wantError: "worker failed", transition: func(b backend.Backend, signature *task.Signature) error {
			return b.SetStateFailure(signature, "worker failed")
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var b backend.Backend
			server, b := newCallbackPublishTestServer(t, func(_ context.Context, signature *task.Signature) error {
				assertPendingAtPublish(t, b, signature)
				if err := tt.transition(b, signature); err != nil {
					t.Fatalf("simulate worker transition: %v", err)
				}
				return publishErr
			})
			signature := task.NewSignature("accepted-before-error-"+tt.name, "callback")

			asyncResult, err := server.SendTask(signature)
			if asyncResult != nil || !errors.Is(err, publishErr) {
				t.Fatalf("send = (%#v, %v), want nil and ambiguous publication error", asyncResult, err)
			}
			assertCallbackTaskState(t, b, signature.ID, tt.wantState, tt.wantError)
		})
	}
}

func TestSendTaskAckLostRacePreservesWorkerState(t *testing.T) {
	publishErr := errors.New("publish acknowledgement lost")
	server, baseBackend := newCallbackPublishTestServer(t, func(context.Context, *task.Signature) error {
		return publishErr
	})
	blockingBackend := &blockingCallbackCompensationBackend{
		Backend: baseBackend,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
	server.backend = blockingBackend
	signature := task.NewSignature("accepted-race", "callback")
	done := make(chan error, 1)
	go func() {
		_, err := server.SendTask(signature)
		done <- err
	}()

	<-blockingBackend.started
	if err := baseBackend.SetStateSuccess(signature, nil); err != nil {
		t.Fatalf("simulate worker success: %v", err)
	}
	close(blockingBackend.release)
	if err := <-done; !errors.Is(err, publishErr) {
		t.Fatalf("SendTask error = %v, want publish error", err)
	}
	assertCallbackTaskState(t, baseBackend, signature.ID, task.StateSuccess, "")
}

func TestOldPublishFailureDoesNotClobberNewAttempt(t *testing.T) {
	publishErr := errors.New("old publish acknowledgement lost")
	oldPublishStarted := make(chan struct{})
	releaseOldPublish := make(chan struct{})
	call := 0
	var b backend.Backend
	server, b := newCallbackPublishTestServer(t, func(_ context.Context, signature *task.Signature) error {
		assertPendingAtPublish(t, b, signature)
		call++
		if call == 1 {
			close(oldPublishStarted)
			<-releaseOldPublish
			return publishErr
		}
		return nil
	})
	oldSignature := task.NewSignature("reused-task-id", "callback")
	newSignature := task.NewSignature("reused-task-id", "callback")
	oldDone := make(chan error, 1)
	go func() {
		_, err := server.SendTask(oldSignature)
		oldDone <- err
	}()
	<-oldPublishStarted
	if result, err := server.SendTask(newSignature); err != nil || result == nil {
		t.Fatalf("new attempt = (%#v, %v), want async result and nil error", result, err)
	}
	close(releaseOldPublish)
	if err := <-oldDone; !errors.Is(err, publishErr) {
		t.Fatalf("old attempt error = %v, want publish error", err)
	}
	assertCallbackTaskState(t, b, newSignature.ID, task.StatePending, "")
}

func TestSendTaskUnsupportedBackendSkipsUnsafeCompensation(t *testing.T) {
	publishErr := errors.New("publish failed")
	b := &unsupportedCompensationBackend{}
	server := &Server{
		backend: b,
		controller: &callbackPublishController{publish: func(context.Context, *task.Signature) error {
			return publishErr
		}},
	}
	if result, err := server.SendTask(task.NewSignature("third-party", "callback")); result != nil || !errors.Is(err, publishErr) {
		t.Fatalf("SendTask = (%#v, %v), want nil and publish error", result, err)
	}
	if b.failureWrites != 0 {
		t.Fatalf("unsafe failure writes = %d, want 0", b.failureWrites)
	}
}

func TestSendTaskAttemptIDGenerationFailureHasNoSideEffects(t *testing.T) {
	generationErr := errors.New("attempt ID generation failed")
	b := &attemptTrackingBackend{}
	publishes := 0
	server := &Server{
		backend: b,
		controller: &callbackPublishController{publish: func(context.Context, *task.Signature) error {
			publishes++
			return nil
		}},
		publicationAttemptID: func() (string, error) { return "", generationErr },
	}
	if result, err := server.SendTask(task.NewSignature("generation-failure", "callback")); result != nil || !errors.Is(err, generationErr) {
		t.Fatalf("SendTask = (%#v, %v), want nil and generation error", result, err)
	}
	if b.pendingAttempts != 0 || len(b.pendingIDs) != 0 || publishes != 0 {
		t.Fatalf("generation failure caused side effects: attempts=%d pending=%v publishes=%d", b.pendingAttempts, b.pendingIDs, publishes)
	}
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

func TestOrdinaryCallbackPublicationErrorsReachErrorHandler(t *testing.T) {
	publishErr := errors.New("callback publish failed")
	compensationErr := errors.New("callback compensation failed")
	parentErr := errors.New("parent task failed")
	tests := []struct {
		name     string
		dispatch func(*Worker, *task.Signature) error
	}{
		{name: "success callback", dispatch: func(worker *Worker, parent *task.Signature) error {
			return worker.handlerSucceeded(parent, nil)
		}},
		{name: "error callback", dispatch: func(worker *Worker, parent *task.Signature) error {
			return worker.handlerFailed(parent, parentErr)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server, baseBackend := newCallbackPublishTestServer(t, func(context.Context, *task.Signature) error {
				return publishErr
			})
			server.backend = &callbackCompensationFailBackend{Backend: baseBackend, err: compensationErr}
			callback := task.NewSignature("reported-callback", "callback")
			callback.Args = []task.Arg{{Type: "string", Value: "sensitive-callback-payload"}}
			parent := task.NewSignature("reported-parent", "parent")
			parent.CallbackOnSuccess = []*task.Signature{callback}
			parent.CallbackOnError = []*task.Signature{callback}
			worker := &Worker{bindService: server}
			var reported []error
			worker.errorHandler = func(err error) { reported = append(reported, err) }

			if err := tt.dispatch(worker, parent); err != nil {
				t.Fatalf("dispatch returned error: %v", err)
			}
			var callbackErr error
			for _, err := range reported {
				if errors.Is(err, publishErr) || errors.Is(err, compensationErr) {
					callbackErr = err
				}
				if strings.Contains(err.Error(), "sensitive-callback-payload") {
					t.Fatalf("error handler received sensitive callback payload: %v", err)
				}
			}
			if callbackErr == nil || !errors.Is(callbackErr, publishErr) || !errors.Is(callbackErr, compensationErr) {
				t.Fatalf("reported errors = %v, want joined callback publication and compensation errors", reported)
			}
		})
	}
}
