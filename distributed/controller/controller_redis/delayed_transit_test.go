package controller_redis

import (
	"context"
	"strings"
	"testing"
	"time"

	json "github.com/json-iterator/go"

	"github.com/go-redis/redis/v8"

	"github.com/songzhibin97/gkit/distributed/task"
)

// dueDelayedTaskBody enqueues an already-due delayed task into the delayed
// zset and returns its serialized body and score.
func dueDelayedTaskBody(t *testing.T, client redis.UniversalClient, id string) ([]byte, float64) {
	t.Helper()
	due := time.Now().Add(-time.Minute)
	signature := task.NewSignature(id, "task", task.SetETATime(&due))
	signature.Router = "test_task"
	body, err := json.Marshal(signature)
	if err != nil {
		t.Fatalf("marshal delayed task: %v", err)
	}
	score := float64(due.UnixNano())
	if err := client.ZAdd(context.Background(), "delayed", &redis.Z{Score: score, Member: body}).Err(); err != nil {
		t.Fatalf("enqueue delayed task: %v", err)
	}
	return body, score
}

// redisHoldsTaskBody scans every key in Redis for the task body, regardless of
// which data structure holds it.
func redisHoldsTaskBody(t *testing.T, client redis.UniversalClient, body string) bool {
	t.Helper()
	ctx := context.Background()
	keys, err := client.Keys(ctx, "*").Result()
	if err != nil {
		t.Fatalf("list redis keys: %v", err)
	}
	for _, key := range keys {
		switch client.Type(ctx, key).Val() {
		case "list":
			for _, item := range client.LRange(ctx, key, 0, -1).Val() {
				if strings.Contains(item, body) {
					return true
				}
			}
		case "zset":
			for _, item := range client.ZRange(ctx, key, 0, -1).Val() {
				if strings.Contains(item, body) {
					return true
				}
			}
		case "set":
			for _, item := range client.SMembers(ctx, key).Val() {
				if strings.Contains(item, body) {
					return true
				}
			}
		case "hash":
			for _, item := range client.HGetAll(ctx, key).Val() {
				if strings.Contains(item, body) {
					return true
				}
			}
		case "string":
			if strings.Contains(client.Get(ctx, key).Val(), body) {
				return true
			}
		}
	}
	return false
}

// TestClaimedDelayedTaskRemainsRecoverableInRedis covers the crash-loss
// window: the delayed producer first removes the due task from the delayed
// zset and only later republishes it to its router queue. If the process
// crashes between the two steps, the task must still exist somewhere in Redis
// so a recovery pass can republish it. The old WATCH+ZRem implementation left
// it nowhere — an ETA task was lost forever.
func TestClaimedDelayedTaskRemainsRecoverableInRedis(t *testing.T) {
	c, _, client := newMiniController(t)
	ctx := context.Background()
	body, _ := dueDelayedTaskBody(t, client, "crash-window")

	// Claim the due task exactly as produceDelayedTasks does, then simulate a
	// crash before Publish by dropping the returned bytes on the floor.
	popped, _, err := c.popDelayedTaskWithContext(ctx, "delayed", 1)
	if err != nil {
		t.Fatalf("pop delayed task: %v", err)
	}
	if string(popped) != string(body) {
		t.Fatalf("popped = %q, want %q", popped, body)
	}
	if remaining := client.ZCard(ctx, "delayed").Val(); remaining != 0 {
		t.Fatalf("delayed zset cardinality = %d, want 0 after claim", remaining)
	}

	if !redisHoldsTaskBody(t, client, string(body)) {
		t.Fatal("claimed delayed task exists nowhere in Redis: a crash before Publish loses it forever")
	}
}

// startDelayedProducer runs produceDelayedTasks in the background and returns
// a stop function that cancels it, joins it, and fails on reported errors.
func startDelayedProducer(t *testing.T, c *ControllerRedis) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	failures := make(chan error, 1)
	done := make(chan struct{})
	go func() {
		defer close(done)
		c.produceDelayedTasks(ctx, "delayed", func(err error) {
			select {
			case failures <- err:
			default:
			}
		})
	}()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Fatal("delayed producer did not stop after cancellation")
		}
		select {
		case err := <-failures:
			t.Fatalf("delayed producer reported failure: %v", err)
		default:
		}
	}
}

func waitForRouterTask(t *testing.T, client redis.UniversalClient, wantID string) {
	t.Helper()
	ctx := context.Background()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if republished := client.LRange(ctx, "test_task", 0, -1).Val(); len(republished) > 0 {
			// Publish re-marshals the signature, so compare decoded IDs.
			var got task.Signature
			if err := json.Unmarshal([]byte(republished[0]), &got); err != nil {
				t.Fatalf("decode republished task: %v", err)
			}
			if got.ID != wantID {
				t.Fatalf("republished task ID = %q, want %q", got.ID, wantID)
			}
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("task %q was never republished to its router queue", wantID)
}

func waitForEmptyTransit(t *testing.T, client redis.UniversalClient) {
	t.Helper()
	ctx := context.Background()
	transit := deriveDelayedTransitKey("delayed")
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if client.ZCard(ctx, transit).Val() == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("transit zset still holds %d entries, want 0", client.ZCard(ctx, transit).Val())
}

// TestDelayedTaskCrashBeforePublishIsRepublishedByRecovery closes the loop:
// after a producer claims a due task and crashes before Publish, a fresh
// producer's recovery pass must move the transit entry back to the delayed
// zset and republish it to its router queue (at-least-once, never lost).
func TestDelayedTaskCrashBeforePublishIsRepublishedByRecovery(t *testing.T) {
	c, mr, client := newMiniController(t)
	ctx := context.Background()
	dueDelayedTaskBody(t, client, "crash-recover")

	// Claim, then simulate a crash before Publish by dropping the bytes.
	if _, _, err := c.popDelayedTaskWithContext(ctx, "delayed", 1); err != nil {
		t.Fatalf("pop delayed task: %v", err)
	}

	// A fresh producer starts after the recovery timeout elapsed (advance the
	// Redis server clock the transit claim scores are anchored to).
	mr.SetTime(time.Now().Add(delayedTransitRecoveryTimeout + time.Second))
	stop := startDelayedProducer(t, c)
	waitForRouterTask(t, client, "crash-recover")
	waitForEmptyTransit(t, client)
	stop()
}

// TestStaleTransitEntryIsRecoveredWhileProducerIsLive covers the recovery
// starvation bug: popDelayedTaskWithContext polls internally while the delayed
// zset is empty (redis.Nil -> continue), so a recovery check at the head of
// the produce loop only ever ran at startup. A live producer with no due
// tasks then never recovered transit entries stranded by a crashed peer.
// Periodic recovery must run on its own ticker, independent of claim traffic.
func TestStaleTransitEntryIsRecoveredWhileProducerIsLive(t *testing.T) {
	c, mr, client := newMiniController(t)
	ctx := context.Background()
	c.delayedRecoveryInterval = 20 * time.Millisecond

	// A sentinel task is claimed and published only after the startup recovery
	// pass, so waiting for it (and for its transit entry to clear) proves the
	// producer is past startup recovery and idle-polling an empty delayed zset
	// before the stale entry is planted.
	dueDelayedTaskBody(t, client, "sentinel")
	stop := startDelayedProducer(t, c)
	waitForRouterTask(t, client, "sentinel")
	waitForEmptyTransit(t, client)
	if err := client.Del(ctx, "test_task").Err(); err != nil {
		t.Fatalf("clear router queue: %v", err)
	}

	// A crashed peer left a claimed task in transit. No new task is published
	// after this point: only the live producer's periodic recovery can move
	// the entry back to the delayed zset for a fresh claim.
	due := time.Now().Add(-time.Minute)
	signature := task.NewSignature("stale-peer", "task", task.SetETATime(&due))
	signature.Router = "test_task"
	body, err := json.Marshal(signature)
	if err != nil {
		t.Fatalf("marshal stale transit task: %v", err)
	}
	claimedAtMs := float64(time.Now().UnixMilli())
	if err := client.ZAdd(ctx, deriveDelayedTransitKey("delayed"), &redis.Z{Score: claimedAtMs, Member: body}).Err(); err != nil {
		t.Fatalf("plant stale transit entry: %v", err)
	}
	// Push the Redis server clock (which anchors transit claim scores) past
	// the staleness threshold so recovery treats the peer as crashed.
	mr.SetTime(time.Now().Add(delayedTransitRecoveryTimeout + time.Second))

	waitForRouterTask(t, client, "stale-peer")
	waitForEmptyTransit(t, client)
	stop()
}

// TestDelayedRepublishFinalizesTransit covers the normal path: a due task is
// republished to its router queue and its transit entry is cleared, so a
// later recovery pass cannot deliver it a second time.
func TestDelayedRepublishFinalizesTransit(t *testing.T) {
	c, _, client := newMiniController(t)
	dueDelayedTaskBody(t, client, "normal-path")

	stop := startDelayedProducer(t, c)
	waitForRouterTask(t, client, "normal-path")
	waitForEmptyTransit(t, client)
	stop()

	if remaining := client.ZCard(context.Background(), "delayed").Val(); remaining != 0 {
		t.Fatalf("delayed zset cardinality = %d, want 0 after republish", remaining)
	}
}
