package distributed

import (
	"context"
	"errors"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend"
	backendredis "github.com/songzhibin97/gkit/distributed/backend/backend_redis"
	"github.com/songzhibin97/gkit/distributed/task"
)

// This wrapper deliberately exposes only Backend, modeling an implementation
// without atomic publication-attempt compensation.
type groupBackendWithoutAttempts struct{ backend.Backend }

type groupGenerationBackend struct {
	attemptTrackingBackend
	takeovers int
}

func (b *groupGenerationBackend) GroupTakeOver(string, string, ...string) error {
	b.takeovers++
	return nil
}

func TestGroupPublicationAttemptGenerationFailureHasNoWrites(t *testing.T) {
	wantErr := errors.New("synthetic attempt generation failure")
	b := &groupGenerationBackend{}
	controller := &groupTestController{}
	server := &Server{backend: b, controller: controller, publicationAttemptID: func() (string, error) { return "", wantErr }}
	if _, err := server.SendGroup(newIssue79Group("t0", "t1"), 1); !errors.Is(err, wantErr) {
		t.Errorf("SendGroup error = %v, want %v", err, wantErr)
	}
	if b.takeovers != 0 || b.pendingAttempts != 0 || len(b.pendingIDs) != 0 || controller.publishCount.Load() != 0 {
		t.Fatalf("generation failure caused writes: takeover=%d pendingAttempts=%d pending=%v publishes=%d", b.takeovers, b.pendingAttempts, b.pendingIDs, controller.publishCount.Load())
	}
}

func newGroupAttemptFixture(t *testing.T) (backend.Backend, *task.Group) {
	t.Helper()
	addr := os.Getenv("GKIT_GROUP_PUBLICATION_REDIS_ADDR")
	if addr == "" {
		addr = miniredis.RunT(t).Addr()
	} else {
		host, _, err := net.SplitHostPort(addr)
		if err != nil || host != "127.0.0.1" {
			t.Fatal("live publication tests require an isolated loopback Redis")
		}
		t.Log("using explicitly configured loopback Redis")
	}
	client := redis.NewClient(&redis.Options{Addr: addr, ReadTimeout: time.Second, WriteTimeout: time.Second})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	b := backendredis.NewBackendRedis(client, -1)
	id, err := generatePublicationAttemptID()
	if err != nil {
		t.Fatal(err)
	}
	signature := task.NewSignature("issue166-member-"+id, "task")
	group, err := task.NewGroup("issue166-group-"+id, "group", signature)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.ResetTask(group.GetTaskIDs()...); err != nil {
			t.Error(err)
		}
		if err := b.ResetGroup(group.GroupID); err != nil {
			t.Error(err)
		}
	})
	return b, group
}

// Regression for #166 / 03-01: Publish can report an ACK failure after the
// actual worker has stored SUCCESS and results. Compensation must preserve it.
func TestGroupPublicationAckLostPreservesActualWorkerSuccess(t *testing.T) {
	for _, useAttempts := range []bool{true, false} {
		name := "attempt_backend"
		if !useAttempts {
			name = "legacy_backend"
		}
		t.Run(name, func(t *testing.T) {
			b, group := newGroupAttemptFixture(t)
			publicationErr := errors.New("synthetic publish acknowledgment lost")
			server := &Server{backend: b, registeredTasks: &sync.Map{}}
			if !useAttempts {
				server.backend = groupBackendWithoutAttempts{Backend: b}
			}
			worker := server.NewWorker("test-worker", 1, "test-queue")
			var executions atomic.Int32
			server.controller = &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
				if err := worker.Process(signature); err != nil {
					return err
				}
				status, err := b.GetStatus(signature.ID)
				if err != nil {
					return err
				}
				if !status.IsSuccess() {
					return errors.New("worker did not complete before ACK loss")
				}
				return publicationErr
			}}
			if err := server.RegisteredTask("task", func() (string, error) {
				executions.Add(1)
				return "synthetic completed result", nil
			}); err != nil {
				t.Fatal(err)
			}
			results, err := server.SendGroup(group, 1)
			if !errors.Is(err, publicationErr) || len(results) != 1 || results[0] != nil {
				t.Fatalf("group result = (%v, %v), want unconfirmed publication and original error", results, err)
			}
			if executions.Load() != 1 {
				t.Fatalf("worker executions = %d, want 1", executions.Load())
			}
			status, err := b.GetStatus(group.Tasks[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			if !status.IsSuccess() || status.Error != "" || len(status.Results) != 1 || status.Results[0].Value != "synthetic completed result" {
				t.Fatalf("worker result overwritten after ACK loss: %#v", status)
			}
		})
	}
}

func TestGroupPublicationFailurePreservesAdvancedOrNewerAttempt(t *testing.T) {
	for _, state := range []task.State{task.StateReceived, task.StateStarted, task.StateRetry, task.StateFailure, task.StatePending} {
		t.Run(state.String(), func(t *testing.T) {
			b, group := newGroupAttemptFixture(t)
			publicationErr := errors.New("synthetic publication outcome unknown")
			server := &Server{backend: b, controller: &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
				var err error
				switch state {
				case task.StateReceived:
					err = b.SetStateReceived(signature)
				case task.StateStarted:
					err = b.SetStateStarted(signature)
				case task.StateRetry:
					err = b.SetStateRetry(signature)
				case task.StateFailure:
					err = b.SetStateFailure(signature, "worker failed")
				case task.StatePending:
					err = b.(backend.PublicationAttemptBackend).SetStatePendingAttempt(signature, "newer-attempt")
				}
				if err != nil {
					return err
				}
				return publicationErr
			}}}
			if _, err := server.SendGroup(group, 1); !errors.Is(err, publicationErr) {
				t.Fatalf("SendGroup error = %v", err)
			}
			status, err := b.GetStatus(group.Tasks[0].ID)
			if err != nil {
				t.Fatal(err)
			}
			wantError := ""
			if state == task.StateFailure {
				wantError = "worker failed"
			}
			if status.Status != state || status.Error != wantError {
				t.Fatalf("advanced state overwritten: %#v, want %v/%q", status, state, wantError)
			}
			if state == task.StatePending {
				changed, err := b.(backend.PublicationAttemptBackend).FailPendingAttempt(group.Tasks[0], "newer-attempt", "new owner control")
				if err != nil || !changed {
					t.Fatalf("new attempt ownership lost: (%t, %v)", changed, err)
				}
			}
		})
	}
}

func TestGroupPublicationFailureReasonDistinguishesAdmission(t *testing.T) {
	b, group := newGroupAttemptFixture(t)
	second := task.NewSignature(group.Tasks[0].ID+"-unadmitted", "task")
	group, err := task.NewGroup(group.GroupID, group.Name, group.Tasks[0], second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := b.ResetTask(second.ID); err != nil {
			t.Error(err)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	server := &Server{backend: b, controller: &groupTestController{publishFn: func(context.Context, *task.Signature) error {
		cancel()
		return context.Canceled
	}}}
	if _, err := server.SendGroupWithContext(ctx, group, 1); !errors.Is(err, context.Canceled) {
		t.Fatalf("SendGroup error = %v", err)
	}
	assertCallbackTaskState(t, b, group.Tasks[0].ID, task.StateFailure, "task publication outcome unknown")
	assertCallbackTaskState(t, b, second.ID, task.StateFailure, "group publication canceled before task execution")
}

func TestGroupPublicationUnsupportedBackendLeavesPending(t *testing.T) {
	b, group := newGroupAttemptFixture(t)
	publicationErr := errors.New("synthetic publication failure")
	server := &Server{backend: groupBackendWithoutAttempts{Backend: b}, controller: &groupTestController{publishFn: func(context.Context, *task.Signature) error { return publicationErr }}}
	if _, err := server.SendGroup(group, 1); !errors.Is(err, publicationErr) {
		t.Fatalf("SendGroup error = %v", err)
	}
	assertCallbackTaskState(t, b, group.Tasks[0].ID, task.StatePending, "")
}
