package controller_redis

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"

	"github.com/songzhibin97/gkit/distributed/broker"
)

func newMiniController(t *testing.T) (*ControllerRedis, *miniredis.Miniredis, redis.UniversalClient) {
	t.Helper()
	mr := miniredis.RunT(t)
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{mr.Addr()}})
	t.Cleanup(func() { _ = client.Close() })
	bk := broker.NewBroker(broker.NewRegisteredTask(), context.Background())
	c := NewControllerRedis(bk, client, "test_task", "delayed").(*ControllerRedis)
	return c, mr, client
}

// TestPopDelayedTask_FIFOByETA covers the FIFO fix: delayed tasks are stored in
// a ZSET scored by ETA, and popDelayedTask must return the EARLIEST-due task
// first (ascending ZRangeByScore). The bug used ZRevRangeByScore (latest first).
func TestPopDelayedTask_FIFOByETA(t *testing.T) {
	c, _, client := newMiniController(t)
	ctx := context.Background()

	// Two due tasks (scores well below now-in-nanos), distinct ETAs.
	if err := client.ZAdd(ctx, "delayed", &redis.Z{Score: 2, Member: "task-B"}).Err(); err != nil {
		t.Fatalf("ZAdd B: %v", err)
	}
	if err := client.ZAdd(ctx, "delayed", &redis.Z{Score: 1, Member: "task-A"}).Err(); err != nil {
		t.Fatalf("ZAdd A: %v", err)
	}

	first, err := c.popDelayedTask("delayed", 1) // 1ns block -> fast
	if err != nil {
		t.Fatalf("pop 1: %v", err)
	}
	if string(first) != "task-A" {
		t.Fatalf("first pop = %q, want task-A (earliest ETA first)", first)
	}
	second, err := c.popDelayedTask("delayed", 1)
	if err != nil {
		t.Fatalf("pop 2: %v", err)
	}
	if string(second) != "task-B" {
		t.Fatalf("second pop = %q, want task-B", second)
	}
}

// TestPopDelayedTask_PropagatesWatchError covers the error-surfacing fix: a real
// Watch/transaction error must propagate, not be swallowed and reported as "no
// task ready" (the old code did `break` + `return result, nil`). We force a
// WRONGTYPE error by making the delayed key a string instead of a ZSET.
func TestPopDelayedTask_PropagatesWatchError(t *testing.T) {
	c, _, client := newMiniController(t)
	ctx := context.Background()

	if err := client.Set(ctx, "delayed", "not-a-zset", 0).Err(); err != nil {
		t.Fatalf("Set: %v", err)
	}

	_, err := c.popDelayedTask("delayed", 1)
	if err == nil {
		t.Fatal("expected a real error to propagate, got nil (error was swallowed)")
	}
	if errors.Is(err, redis.Nil) {
		t.Fatalf("error reported as redis.Nil (swallowed): %v", err)
	}
}
