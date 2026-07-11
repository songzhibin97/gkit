package controller_redis

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"

	"github.com/songzhibin97/gkit/distributed/backend/backend_redis"
	"github.com/songzhibin97/gkit/distributed/broker"
	"github.com/songzhibin97/gkit/distributed/locker/lock_redis"
	"github.com/songzhibin97/gkit/distributed/task"
)

func TestStopConsumingKeepsSharedRedisClientUsable(t *testing.T) {
	mr := miniredis.RunT(t)
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{mr.Addr()}})
	t.Cleanup(func() { _ = client.Close() })

	backend := backend_redis.NewBackendRedis(client, -1)
	locker := lock_redis.NewRedisLock(client)
	controller := NewControllerRedis(
		broker.NewBroker(broker.NewRegisteredTask(), context.Background()),
		client,
		"tasks",
		"delayed",
	)
	controller.StopConsuming()

	t.Run("client", func(t *testing.T) {
		assertSharedRedisOperation(t, "ping", client.Ping(context.Background()).Err())
	})
	t.Run("backend", func(t *testing.T) {
		signature := task.NewSignature("shared-client-task", "task")
		assertSharedRedisOperation(t, "set pending state", backend.SetStatePending(signature))
	})
	t.Run("locker", func(t *testing.T) {
		assertSharedRedisOperation(t, "lock", locker.Lock("shared-client-lock", 1000, "owner"))
	})
}

func assertSharedRedisOperation(t *testing.T, operation string, err error) {
	t.Helper()
	if errors.Is(err, redis.ErrClosed) {
		t.Fatalf("%s failed because StopConsuming closed the shared Redis client: %v", operation, err)
	}
	if err != nil {
		t.Fatalf("%s failed: %v", operation, err)
	}
}
