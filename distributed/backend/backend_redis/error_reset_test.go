package backend_redis

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/task"
)

type publicationAttemptBackendForTest interface {
	SetStatePendingAttempt(*task.Signature, string) error
	FailPendingAttempt(*task.Signature, string, string) (bool, error)
}

func TestNonFailureTransitionsClearPreviousError(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	backend := NewBackendRedis(client, -1)
	tests := []struct {
		name       string
		wantState  task.State
		transition func(*task.Signature) error
	}{
		{name: "pending", wantState: task.StatePending, transition: backend.SetStatePending},
		{name: "received", wantState: task.StateReceived, transition: backend.SetStateReceived},
		{name: "started", wantState: task.StateStarted, transition: backend.SetStateStarted},
		{name: "retry", wantState: task.StateRetry, transition: backend.SetStateRetry},
		{name: "success", wantState: task.StateSuccess, transition: func(sig *task.Signature) error { return backend.SetStateSuccess(sig, nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature := &task.Signature{ID: "task-" + tt.name, GroupID: "group", Name: "task"}
			if err := backend.SetStateFailure(signature, "stale failure"); err != nil {
				t.Fatal(err)
			}
			failed, err := backend.GetStatus(signature.ID)
			if err != nil {
				t.Fatal(err)
			}
			if failed.Status != task.StateFailure || failed.Error != "stale failure" {
				t.Fatalf("failure state = (%s, %q), want (FAILURE, stale failure)", failed.Status, failed.Error)
			}
			if err := tt.transition(signature); err != nil {
				t.Fatal(err)
			}
			status, err := backend.GetStatus(signature.ID)
			if err != nil {
				t.Fatal(err)
			}
			if status.Status != tt.wantState {
				t.Fatalf("status = %s, want %s", status.Status, tt.wantState)
			}
			if status.Error != "" {
				t.Fatalf("%s error = %q, want cleared", tt.name, status.Error)
			}
		})
	}
}

func TestPublicationAttemptCompensation(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	b := NewBackendRedis(client, 60)
	attemptBackend, ok := b.(publicationAttemptBackendForTest)
	if !ok {
		t.Fatal("BackendRedis does not implement atomic publication-attempt compensation")
	}
	signature := &task.Signature{ID: "publication-attempt", GroupID: "group", Name: "task"}
	if err := attemptBackend.SetStatePendingAttempt(signature, "attempt-a"); err != nil {
		t.Fatal(err)
	}
	ttlBefore := mr.TTL(signature.ID)
	if changed, err := attemptBackend.FailPendingAttempt(signature, "attempt-a", "publish failed"); err != nil || !changed {
		t.Fatalf("matching compensation = (%t, %v), want true, nil", changed, err)
	}
	if ttlAfter := mr.TTL(signature.ID); ttlAfter != ttlBefore || ttlAfter <= 0 || ttlAfter > time.Minute {
		t.Fatalf("TTL after compensation = %s, before %s", ttlAfter, ttlBefore)
	}
	status, err := b.GetStatus(signature.ID)
	if err != nil || status.Status != task.StateFailure || status.Error != "publish failed" {
		t.Fatalf("matching status = %#v, %v", status, err)
	}

	if err := attemptBackend.SetStatePendingAttempt(signature, "attempt-b"); err != nil {
		t.Fatal(err)
	}
	if changed, err := attemptBackend.FailPendingAttempt(signature, "attempt-a", "stale attempt"); err != nil || changed {
		t.Fatalf("stale compensation = (%t, %v), want false, nil", changed, err)
	}
	status, err = b.GetStatus(signature.ID)
	if err != nil || status.Status != task.StatePending || status.Error != "" {
		t.Fatalf("stale-attempt status = %#v, %v", status, err)
	}
}

func TestPublicationAttemptCompensationLegacyRecordDoesNotMatch(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })
	b := NewBackendRedis(client, -1)
	attemptBackend, ok := b.(publicationAttemptBackendForTest)
	if !ok {
		t.Fatal("BackendRedis does not implement atomic publication-attempt compensation")
	}
	signature := &task.Signature{ID: "legacy-pending", GroupID: "group", Name: "task"}
	if err := attemptBackend.SetStatePendingAttempt(signature, "stale-owner"); err != nil {
		t.Fatal(err)
	}
	if err := b.SetStatePending(signature); err != nil {
		t.Fatal(err)
	}
	if changed, err := attemptBackend.FailPendingAttempt(signature, "stale-owner", "publish failed"); err != nil || changed {
		t.Fatalf("legacy compensation = (%t, %v), want false, nil", changed, err)
	}
	status, err := b.GetStatus(signature.ID)
	if err != nil || status.Status != task.StatePending {
		t.Fatalf("legacy status = %#v, %v", status, err)
	}
}
