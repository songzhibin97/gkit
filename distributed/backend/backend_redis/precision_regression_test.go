package backend_redis

import (
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend/chordtest"
	"os"
	"testing"
)

func TestIssue167ResultPrecision(t *testing.T) {
	addr := os.Getenv("GKIT_REDIS_ADDR")
	if addr == "" {
		t.Skip("#167 live precision requires GKIT_REDIS_ADDR; owned fixture required before acceptance")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	chordtest.RunResultPrecision(t, NewBackendRedis(client, -1).(*BackendRedis))
}

func TestIssue167ResultConsumers(t *testing.T) {
	addr := os.Getenv("GKIT_REDIS_ADDR")
	if addr == "" {
		t.Skip("#167 live precision requires GKIT_REDIS_ADDR; owned fixture required before acceptance")
	}
	client := redis.NewClient(&redis.Options{Addr: addr})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	chordtest.RunResultConsumers(t, NewBackendRedis(client, -1).(*BackendRedis))
}
