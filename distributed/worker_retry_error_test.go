package distributed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
	"github.com/songzhibin97/gkit/log"
)

type concreteWorkerRetry struct{ delay time.Duration }

func (e concreteWorkerRetry) Error() string          { return "synthetic worker retry" }
func (e concreteWorkerRetry) RetryIn() time.Duration { return e.delay }

type workerRetryStateBackend struct {
	backend.Backend
	backend.PublicationAttemptBackend
	retryStates []task.State
}

func (b *workerRetryStateBackend) SetStateRetry(signature *task.Signature) error {
	if err := b.Backend.SetStateRetry(signature); err != nil {
		return err
	}
	status, err := b.Backend.GetStatus(signature.ID)
	if err != nil {
		return err
	}
	b.retryStates = append(b.retryStates, status.Status)
	return nil
}

func TestWorkerRetriesActualConcreteAndWrappedErrors(t *testing.T) {
	const delay = time.Minute
	custom := concreteWorkerRetry{delay: delay}
	standard := task.NewErrRetryTaskLater("synthetic standard retry", delay)
	for _, test := range []struct {
		name string
		fn   interface{}
	}{
		{"custom_value", func() concreteWorkerRetry { return custom }},
		{"custom_pointer", func() *concreteWorkerRetry { return &custom }},
		{"custom_interface", func() error { return custom }},
		{"wrapped", func() error { return fmt.Errorf("wrapped retry: %w", custom) }},
		{"standard_value", func() task.ErrRetryTaskLater { return standard }},
		{"standard_pointer", func() *task.ErrRetryTaskLater { return &standard }},
		{"standard_interface", func() error { return standard }},
	} {
		t.Run(test.name, func(t *testing.T) {
			base, group := newGroupAttemptFixture(t)
			b := &workerRetryStateBackend{Backend: base, PublicationAttemptBackend: base.(backend.PublicationAttemptBackend)}
			signature := group.Tasks[0]
			signature.RetryCount = 0 // RetryIn must work without the generic retry budget.
			var published *task.Signature
			controller := &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
				published = task.CopySignature(signature)
				return nil
			}}
			server := &Server{backend: b, controller: controller, registeredTasks: &sync.Map{}, helper: log.NewHelper(log.NewStdLogger(io.Discard))}
			if err := server.RegisteredTask(signature.Name, test.fn); err != nil {
				t.Fatal(err)
			}
			if err := b.SetStatePending(signature); err != nil {
				t.Fatal(err)
			}
			worker := server.NewWorker("test-retry", 1, "test-queue")
			var reported error
			worker.SetErrorHandler(func(err error) { reported = err })
			before := time.Now()
			if err := worker.Process(signature); err != nil {
				t.Fatalf("Process: %v", err)
			}
			after := time.Now()
			if reported != nil {
				t.Errorf("retry error incorrectly reported as terminal: %v", reported)
			}
			if len(b.retryStates) != 1 || b.retryStates[0] != task.StateRetry {
				t.Errorf("actual backend retry transitions = %v, want one RETRY", b.retryStates)
			}
			if controller.publishCount.Load() != 1 || published == nil || published.ETA == nil {
				t.Fatalf("retry was not republished: count=%d signature=%v", controller.publishCount.Load(), published)
			}
			if published.ID != signature.ID || published.ETA.Before(before.Add(delay)) || published.ETA.After(after.Add(delay)) {
				t.Errorf("retry publication ID/ETA changed: id=%s ETA=%v", published.ID, published.ETA)
			}
			assertCallbackTaskState(t, base, signature.ID, task.StatePending, "")
		})
	}
}

func TestWorkerOrdinaryErrorKeepsRetryCountPolicy(t *testing.T) {
	for _, retries := range []int{0, 1} {
		t.Run(fmt.Sprintf("retry_count_%d", retries), func(t *testing.T) {
			base, group := newGroupAttemptFixture(t)
			b := &workerRetryStateBackend{Backend: base, PublicationAttemptBackend: base.(backend.PublicationAttemptBackend)}
			signature := group.Tasks[0]
			signature.RetryCount, signature.RetryInterval = retries, 1
			wantErr := errors.New("synthetic ordinary failure")
			var published *task.Signature
			controller := &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
				published = task.CopySignature(signature)
				return nil
			}}
			server := &Server{backend: b, controller: controller, registeredTasks: &sync.Map{}, helper: log.NewHelper(log.NewStdLogger(io.Discard))}
			if err := server.RegisteredTask(signature.Name, func() error { return wantErr }); err != nil {
				t.Fatal(err)
			}
			if err := b.SetStatePending(signature); err != nil {
				t.Fatal(err)
			}
			worker := server.NewWorker("ordinary-error", 1, "test-queue")
			var reported error
			worker.SetErrorHandler(func(err error) { reported = err })
			before := time.Now()
			if err := worker.Process(signature); err != nil {
				t.Fatal(err)
			}
			after := time.Now()
			if retries == 0 {
				if reported != wantErr || published != nil || len(b.retryStates) != 0 {
					t.Fatalf("ordinary failure took retry path: reported=%v published=%v states=%v", reported, published, b.retryStates)
				}
				assertCallbackTaskState(t, base, signature.ID, task.StateFailure, wantErr.Error())
				return
			}
			if reported != nil || controller.publishCount.Load() != 1 || published == nil || published.ETA == nil || published.RetryCount != 0 || published.RetryInterval != 2 {
				t.Fatalf("ordinary retry policy changed: reported=%v published=%v", reported, published)
			}
			if len(b.retryStates) != 1 || b.retryStates[0] != task.StateRetry {
				t.Fatalf("actual retry transitions = %v", b.retryStates)
			}
			if published.ETA.Before(before.Add(2*time.Second)) || published.ETA.After(after.Add(2*time.Second)) {
				t.Fatalf("generic retry ETA = %v", published.ETA)
			}
			assertCallbackTaskState(t, base, signature.ID, task.StatePending, "")
		})
	}
}
