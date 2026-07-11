package result

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
)

const (
	testTimeout    = 10 * time.Millisecond
	testPoll       = 120 * time.Millisecond
	testUpperBound = 100 * time.Millisecond
)

type pendingResultBackend struct{}

func (*pendingResultBackend) GroupTakeOver(string, string, ...string) error { return nil }
func (*pendingResultBackend) GroupCompleted(string) (bool, error)           { return false, nil }
func (*pendingResultBackend) GroupTaskStatus(string) ([]*task.Status, error) {
	return nil, nil
}
func (*pendingResultBackend) TriggerCompleted(string) (bool, error) { return false, nil }
func (*pendingResultBackend) SetStatePending(*task.Signature) error { return nil }
func (*pendingResultBackend) SetStateReceived(*task.Signature) error {
	return nil
}
func (*pendingResultBackend) SetStateStarted(*task.Signature) error { return nil }
func (*pendingResultBackend) SetStateRetry(*task.Signature) error   { return nil }
func (*pendingResultBackend) SetStateSuccess(*task.Signature, []*task.Result) error {
	return nil
}
func (*pendingResultBackend) SetStateFailure(*task.Signature, string) error { return nil }
func (*pendingResultBackend) GetStatus(taskID string) (*task.Status, error) {
	return &task.Status{TaskID: taskID, Status: task.StatePending}, nil
}
func (*pendingResultBackend) ResetTask(...string) error  { return nil }
func (*pendingResultBackend) ResetGroup(...string) error { return nil }
func (*pendingResultBackend) SetResultExpire(int64)      {}

func TestAsyncResultGetWithTimeoutHonorsDeadlineDuringPollWait(t *testing.T) {
	b := &pendingResultBackend{}
	result := NewAsyncResult(&task.Signature{ID: "async"}, b)
	assertTimeoutBounded(t, result.GetWithTimeout)
}

func TestChainAsyncResultGetWithTimeoutHonorsDeadlineDuringPollWait(t *testing.T) {
	b := &pendingResultBackend{}
	result := NewChainAsyncResult([]*task.Signature{{ID: "chain"}}, b)
	assertTimeoutBounded(t, result.GetWithTimeout)
}

func TestGroupCallbackAsyncResultGetWithTimeoutHonorsDeadlineDuringPollWait(t *testing.T) {
	b := &pendingResultBackend{}
	result := NewGroupCallbackAsyncResult(
		[]*task.Signature{{ID: "group"}},
		&task.Signature{ID: "callback"},
		b,
	)
	assertTimeoutBounded(t, result.GetWithTimeout)
}

func assertTimeoutBounded(t *testing.T, get func(time.Duration, time.Duration) ([]reflect.Value, error)) {
	t.Helper()
	started := time.Now()
	values, err := get(testTimeout, testPoll)
	elapsed := time.Since(started)
	if values != nil {
		t.Fatalf("GetWithTimeout values = %v, want nil", values)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("GetWithTimeout error = %v, want context.DeadlineExceeded", err)
	}
	if elapsed >= testUpperBound {
		t.Fatalf("GetWithTimeout elapsed = %v, want < %v", elapsed, testUpperBound)
	}
}
