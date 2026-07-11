package backend_redis

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/task"
)

func TestPublicationAttemptCompensationRealRedis(t *testing.T) {
	addr := os.Getenv("GKIT_REDIS_ADDR")
	if addr == "" {
		t.Skip("GKIT_REDIS_ADDR is not set")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() { _ = client.Close() })
	if err := client.Ping(context.Background()).Err(); err != nil {
		t.Fatal(err)
	}
	backend := NewBackendRedis(client, 60).(*BackendRedis)
	signature := &task.Signature{
		ID:      fmt.Sprintf("gkit-publication-attempt-%d", time.Now().UnixNano()),
		GroupID: "group",
		Name:    "task",
	}
	t.Cleanup(func() { _ = client.Del(context.Background(), signature.ID).Err() })

	if err := backend.SetStatePendingAttempt(signature, "attempt-a"); err != nil {
		t.Fatal(err)
	}
	ttlBefore, err := client.PTTL(context.Background(), signature.ID).Result()
	if err != nil {
		t.Fatal(err)
	}
	if changed, err := backend.FailPendingAttempt(signature, "attempt-a", "publish failed"); err != nil || !changed {
		t.Fatalf("matching compensation = (%t, %v), want true, nil", changed, err)
	}
	ttlAfter, err := client.PTTL(context.Background(), signature.ID).Result()
	if err != nil {
		t.Fatal(err)
	}
	if ttlAfter <= 0 || ttlAfter > ttlBefore || ttlBefore-ttlAfter > time.Second {
		t.Fatalf("TTL changed unexpectedly: before=%s after=%s", ttlBefore, ttlAfter)
	}
	if err := backend.SetStatePendingAttempt(signature, "attempt-b"); err != nil {
		t.Fatal(err)
	}
	if changed, err := backend.FailPendingAttempt(signature, "attempt-a", "stale attempt"); err != nil || changed {
		t.Fatalf("stale compensation = (%t, %v), want false, nil", changed, err)
	}
	status, err := backend.GetStatus(signature.ID)
	if err != nil || status.Status != task.StatePending || status.Error != "" {
		t.Fatalf("final status = %#v, %v", status, err)
	}
}
