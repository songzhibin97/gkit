package distributed

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend"
	backendredis "github.com/songzhibin97/gkit/distributed/backend/backend_redis"
	"github.com/songzhibin97/gkit/distributed/task"
)

type cancelAfterPendingBackend struct {
	backend.Backend
	cancel context.CancelFunc
	target int
	seen   int
}

type failingGroupConvergenceBackend struct {
	groupTestBackend
	failureErr error
}

func (b *failingGroupConvergenceBackend) SetStateFailure(*task.Signature, string) error {
	return b.failureErr
}

func (b *cancelAfterPendingBackend) SetStatePending(signature *task.Signature) error {
	if err := b.Backend.SetStatePending(signature); err != nil {
		return err
	}
	b.seen++
	if b.seen == b.target {
		b.cancel()
	}
	return nil
}

func newGroupConvergenceRedisBackend(t *testing.T) backend.Backend {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	return backendredis.NewBackendRedis(client, -1)
}

func TestSendGroupCancellationBeforeAdmissionRollsBackForRetry(t *testing.T) {
	baseBackend := newGroupConvergenceRedisBackend(t)
	ctx, cancel := context.WithCancel(context.Background())
	wrappedBackend := &cancelAfterPendingBackend{
		Backend: baseBackend,
		cancel:  cancel,
		target:  2,
	}
	server := &Server{
		config:     &Config{ConsumeQueue: "group-convergence"},
		backend:    wrappedBackend,
		controller: &groupTestController{},
	}
	group := newIssue79Group("t0", "t1")

	results, err := server.SendGroupWithContext(ctx, group, 1)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("SendGroupWithContext error = %v, want context.Canceled", err)
	}
	for index, result := range results {
		if result != nil {
			t.Fatalf("result[%d] = %#v, want nil before publisher admission", index, result)
		}
	}
	for _, taskID := range group.GetTaskIDs() {
		if status, statusErr := baseBackend.GetStatus(taskID); !errors.Is(statusErr, redis.Nil) {
			t.Fatalf("status %s after zero-admission cancellation = %#v, %v; want removed", taskID, status, statusErr)
		}
	}

	retryResults, err := server.SendGroupWithContext(context.Background(), newIssue79Group("t0", "t1"), 1)
	if err != nil {
		t.Fatalf("same-group retry after zero-admission cancellation failed: %v", err)
	}
	for index, result := range retryResults {
		if result == nil {
			t.Fatalf("retry result[%d] is nil", index)
		}
	}
}

func TestSendGroupPartialCancellationConvergesUnpublishedMembers(t *testing.T) {
	b := newGroupConvergenceRedisBackend(t)
	started := make(chan struct{})
	release := make(chan struct{})
	controller := &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
		if signature.ID != "t0" {
			return nil
		}
		close(started)
		<-release
		return b.SetStateSuccess(signature, nil)
	}}
	server := &Server{
		config:     &Config{ConsumeQueue: "group-convergence"},
		backend:    b,
		controller: controller,
	}
	group := newIssue79Group("t0", "t1", "t2")
	ctx, cancel := context.WithCancel(context.Background())
	done := sendGroupAsync(server, ctx, group, 1)

	<-started
	cancel()
	close(release)
	outcome := <-done
	if !errors.Is(outcome.err, context.Canceled) {
		t.Fatalf("SendGroupWithContext error = %v, want context.Canceled", outcome.err)
	}
	if got := controller.publishCount.Load(); got != 1 {
		t.Fatalf("publish count = %d, want 1", got)
	}
	if outcome.results[0] == nil || outcome.results[1] != nil || outcome.results[2] != nil {
		t.Fatalf("results = %#v, want only the confirmed publisher populated", outcome.results)
	}

	assertGroupTaskState(t, b, "t0", task.StateSuccess)
	assertGroupTaskState(t, b, "t1", task.StateFailure)
	assertGroupTaskState(t, b, "t2", task.StateFailure)
	if completed, err := b.GroupCompleted(group.GroupID); err != nil || !completed {
		t.Fatalf("GroupCompleted = %v, %v; want true after convergence", completed, err)
	}
}

func TestSendGroupPublishFailureConvergesWithoutOverwritingSuccess(t *testing.T) {
	b := newGroupConvergenceRedisBackend(t)
	publishErr := errors.New("publish t0 failed")
	controller := &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
		if signature.ID == "t0" {
			return publishErr
		}
		return b.SetStateSuccess(signature, nil)
	}}
	server := &Server{
		config:     &Config{ConsumeQueue: "group-convergence"},
		backend:    b,
		controller: controller,
	}
	group := newIssue79Group("t0", "t1")

	results, err := server.SendGroupWithContext(context.Background(), group, 2)
	if !errors.Is(err, publishErr) {
		t.Fatalf("SendGroupWithContext error = %v, want publish failure", err)
	}
	if results[0] != nil || results[1] == nil {
		t.Fatalf("results = %#v, want only successful publisher populated", results)
	}
	assertGroupTaskState(t, b, "t0", task.StateFailure)
	assertGroupTaskState(t, b, "t1", task.StateSuccess)
	if completed, err := b.GroupCompleted(group.GroupID); err != nil || !completed {
		t.Fatalf("GroupCompleted = %v, %v; want true after convergence", completed, err)
	}
}

func TestSendGroupConvergenceFailurePreservesBothErrors(t *testing.T) {
	publishErr := errors.New("publish failed")
	convergenceErr := errors.New("persist failure state failed")
	server := &Server{
		config:  &Config{ConsumeQueue: "group-convergence"},
		backend: &failingGroupConvergenceBackend{failureErr: convergenceErr},
		controller: &groupTestController{publishFn: func(context.Context, *task.Signature) error {
			return publishErr
		}},
	}

	_, err := server.SendGroupWithContext(context.Background(), newIssue79Group("t0"), 1)
	if !errors.Is(err, publishErr) {
		t.Fatalf("error = %v, want original publish failure", err)
	}
	if !errors.Is(err, convergenceErr) {
		t.Fatalf("error = %v, want convergence failure", err)
	}
}

func assertGroupTaskState(t *testing.T, b backend.Backend, taskID string, want task.State) {
	t.Helper()
	status, err := b.GetStatus(taskID)
	if err != nil {
		t.Fatalf("GetStatus(%s): %v", taskID, err)
	}
	if status.Status != want {
		t.Fatalf("status %s = %s, want %s", taskID, status.Status, want)
	}
}
