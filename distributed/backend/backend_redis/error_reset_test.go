package backend_redis

import (
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/task"
)

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
