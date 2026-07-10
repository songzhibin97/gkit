package distributed

import (
	"context"
	"math"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"github.com/songzhibin97/gkit/log"
)

type retryTestBackend struct{}

func (*retryTestBackend) GroupTakeOver(string, string, ...string) error         { return nil }
func (*retryTestBackend) GroupCompleted(string) (bool, error)                   { return false, nil }
func (*retryTestBackend) GroupTaskStatus(string) ([]*task.Status, error)        { return nil, nil }
func (*retryTestBackend) TriggerCompleted(string) (bool, error)                 { return false, nil }
func (*retryTestBackend) SetStatePending(*task.Signature) error                 { return nil }
func (*retryTestBackend) SetStateReceived(*task.Signature) error                { return nil }
func (*retryTestBackend) SetStateStarted(*task.Signature) error                 { return nil }
func (*retryTestBackend) SetStateRetry(*task.Signature) error                   { return nil }
func (*retryTestBackend) SetStateSuccess(*task.Signature, []*task.Result) error { return nil }
func (*retryTestBackend) SetStateFailure(*task.Signature, string) error         { return nil }
func (*retryTestBackend) GetStatus(string) (*task.Status, error)                { return nil, nil }
func (*retryTestBackend) ResetTask(...string) error                             { return nil }
func (*retryTestBackend) ResetGroup(...string) error                            { return nil }
func (*retryTestBackend) SetResultExpire(int64)                                 {}

type retryTestController struct {
	publishedETA *time.Time
}

func (*retryTestController) RegisterTask(...string)                           {}
func (*retryTestController) IsRegisterTask(string) bool                       { return true }
func (*retryTestController) StartConsuming(int, task.Processor) (bool, error) { return false, nil }
func (*retryTestController) StopConsuming()                                   {}
func (c *retryTestController) Publish(_ context.Context, signature *task.Signature) error {
	c.publishedETA = signature.ETA
	return nil
}
func (*retryTestController) GetPendingTasks(string) ([]*task.Signature, error) { return nil, nil }
func (*retryTestController) GetDelayedTasks() ([]*task.Signature, error)       { return nil, nil }
func (*retryTestController) SetConsumingQueue(string)                          {}
func (*retryTestController) SetDelayedQueue(string)                            {}

func TestHandlerRetryClampsExtremeIntervalBeforeDurationConversion(t *testing.T) {
	controller := &retryTestController{}
	server := &Server{
		backend:    &retryTestBackend{},
		controller: controller,
		helper:     log.NewHelper(log.DefaultLogger),
	}
	worker := &Worker{bindService: server}
	signature := &task.Signature{
		ID:            "task-id",
		Name:          "task-name",
		RetryCount:    1,
		RetryInterval: math.MaxInt,
	}

	before := time.Now()
	if err := worker.handlerRetry(signature); err != nil {
		t.Fatalf("handlerRetry returned error: %v", err)
	}

	wantSeconds := int64(math.MaxInt)
	if wantSeconds > maxRetryDelaySeconds {
		wantSeconds = maxRetryDelaySeconds
	}
	if int64(signature.RetryInterval) != wantSeconds {
		t.Fatalf("RetryInterval = %d, want clamped %d seconds", signature.RetryInterval, wantSeconds)
	}
	if signature.ETA == nil || !signature.ETA.After(before) {
		t.Fatalf("ETA = %v, want a future time after %v", signature.ETA, before)
	}
	if controller.publishedETA != signature.ETA {
		t.Fatalf("published ETA = %p, signature ETA = %p", controller.publishedETA, signature.ETA)
	}
}
