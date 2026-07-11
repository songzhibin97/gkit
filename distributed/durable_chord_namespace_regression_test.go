package distributed

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend"
	backendredis "github.com/songzhibin97/gkit/distributed/backend/backend_redis"
	"github.com/songzhibin97/gkit/distributed/task"
	"github.com/songzhibin97/gkit/log"
)

func TestRedisDurableChordNamespaceIsolatedAcrossSendWorkerAndRestart(t *testing.T) {
	t.Run("public send", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = client.Close() })
		server, controller := startNamespaceServer(t, client)
		t.Cleanup(func() { shutdownNamespaceServer(t, server) })

		for _, tt := range []struct {
			name        string
			groupID     string
			memberID    string
			callbackID  string
			wantContext string
		}{
			{name: "group id", groupID: "gkit:chord:{public-group}:record", memberID: "member-a", callbackID: "callback-a", wantContext: "group"},
			{name: "member id", groupID: "group-b", memberID: "gkit:chord:{public-member}:record", callbackID: "callback-b", wantContext: "member"},
			{name: "callback id", groupID: "group-c", memberID: "member-c", callbackID: "gkit:chord:{public-callback}:record", wantContext: "callback"},
		} {
			t.Run(tt.name, func(t *testing.T) {
				groupCallback := namespaceGroupCallback(tt.groupID, tt.memberID, tt.callbackID)
				if _, err := server.SendGroupCallbackWithContext(context.Background(), groupCallback, 1); !errors.Is(err, backend.ErrChordInvalidInput) || !strings.Contains(err.Error(), tt.wantContext) {
					t.Fatalf("SendGroupCallback error = %v, want typed %s context", err, tt.wantContext)
				}
				if got := controller.publishCount.Load(); got != 0 {
					t.Fatalf("reserved identifier published %d tasks", got)
				}
			})
		}
	})

	t.Run("worker status", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = client.Close() })
		server, _ := startNamespaceServer(t, client)
		t.Cleanup(func() { shutdownNamespaceServer(t, server) })
		var handlerCalls atomic.Int32
		if err := server.RegisteredTask("namespace-worker", func() (string, error) {
			handlerCalls.Add(1)
			return "ok", nil
		}); err != nil {
			t.Fatal(err)
		}
		const taskID = "gkit:chord:{worker-status}:record"
		const legacyValue = "legacy-worker-value"
		if err := client.Set(context.Background(), taskID, legacyValue, 0).Err(); err != nil {
			t.Fatal(err)
		}
		worker := server.NewWorker("namespace", 1, "queue")
		err := worker.Process(&task.Signature{ID: taskID, Name: "namespace-worker"})
		if !errors.Is(err, backend.ErrChordInvalidInput) || !strings.Contains(err.Error(), "task") {
			t.Fatalf("worker error = %v, want typed reserved task error", err)
		}
		if handlerCalls.Load() != 0 {
			t.Fatal("worker invoked handler after reserved status-key rejection")
		}
		if got := client.Get(context.Background(), taskID).Val(); got != legacyValue {
			t.Fatalf("worker changed legacy value to %q", got)
		}
	})

	t.Run("legacy collision restart", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = client.Close() })
		groupCallback := namespaceGroupCallback("restart-group", "restart-member", "restart-callback")
		recordKey := namespaceRecordKey(groupCallback.Group.GroupID, groupCallback.Callback.ID)
		const legacyValue = "legacy-non-json-value"
		if err := client.Set(context.Background(), recordKey, legacyValue, 0).Err(); err != nil {
			t.Fatal(err)
		}

		first, firstController := startNamespaceServer(t, client)
		if _, err := first.SendGroupCallbackWithContext(context.Background(), groupCallback, 1); !errors.Is(err, backend.ErrChordRegistrationConflict) {
			t.Fatalf("first send error = %v, want ErrChordRegistrationConflict", err)
		}
		if firstController.publishCount.Load() != 0 || client.Get(context.Background(), recordKey).Val() != legacyValue {
			t.Fatal("first send published work or overwrote legacy record")
		}
		shutdownNamespaceServer(t, first)

		second, secondController := startNamespaceServer(t, client)
		t.Cleanup(func() { shutdownNamespaceServer(t, second) })
		if _, err := second.SendGroupCallbackWithContext(context.Background(), groupCallback, 1); !errors.Is(err, backend.ErrChordRegistrationConflict) {
			t.Fatalf("send after restart error = %v, want ErrChordRegistrationConflict", err)
		}
		if secondController.publishCount.Load() != 0 || client.Get(context.Background(), recordKey).Val() != legacyValue {
			t.Fatal("restart published work or overwrote legacy record")
		}
	})
}

func startNamespaceServer(t *testing.T, client redis.UniversalClient) (*Server, *groupTestController) {
	t.Helper()
	controller := &groupTestController{}
	server, err := NewServerE(
		controller,
		backendredis.NewBackendRedis(client, -1),
		timedGroupTestLocker{},
		log.NewHelper(log.DefaultLogger),
		nil,
		SetEnableDurableChordRegistration(true),
		SetConsumeQueue("namespace-queue"),
	)
	if err != nil {
		t.Fatalf("NewServerE: %v", err)
	}
	return server, controller
}

func shutdownNamespaceServer(t *testing.T, server *Server) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
}

func namespaceGroupCallback(groupID, memberID, callbackID string) *task.GroupCallback {
	return &task.GroupCallback{
		Group:    &task.Group{GroupID: groupID, Name: "namespace", Tasks: []*task.Signature{{ID: memberID, Name: "member"}}},
		Callback: &task.Signature{ID: callbackID, Name: "callback"},
	}
}

func namespaceRecordKey(groupID, callbackID string) string {
	parts := strings.Split(backend.ChordDeliveryKey(groupID, callbackID), ":")
	return "gkit:chord:{" + parts[2] + "}:record"
}
