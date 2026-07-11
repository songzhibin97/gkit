package controller_redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

func newLiveReliableClient(t *testing.T) redis.UniversalClient {
	t.Helper()
	address := os.Getenv("GKIT_REDIS_TEST_ADDR")
	if address == "" {
		t.Skip("GKIT_REDIS_TEST_ADDR is not set")
	}
	client := redis.NewUniversalClient(&redis.UniversalOptions{Addrs: []string{address}})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Fatalf("connect to live Redis: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func cleanupReliableQueue(t *testing.T, client redis.UniversalClient, queue string) {
	t.Helper()
	keys := deriveReliableQueueKeys(queue)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Del(ctx, queue, keys.inflight, keys.visibility, keys.outcomes).Err(); err != nil {
		t.Errorf("cleanup reliable queue: %v", err)
	}
}

func TestReliableLuaRedis7Transitions(t *testing.T) {
	client := newLiveReliableClient(t)
	for index, queue := range []string{
		fmt.Sprintf("gkit:103:live:%d:{tagged}", time.Now().UnixNano()),
		fmt.Sprintf("gkit:103:live:untagged:%d", time.Now().UnixNano()),
		fmt.Sprintf("gkit:103:live:{malformed:%d", time.Now().UnixNano()),
	} {
		t.Run(fmt.Sprintf("queue-%d", index), func(t *testing.T) {
			defer cleanupReliableQueue(t, client, queue)
			q := newReliableQueue(client, queue, 100*time.Millisecond, newDeliveryTokenGenerator(nil))
			payload := []byte{0, byte(index), 255, '{', '}'}
			if err := client.RPush(context.Background(), queue, payload).Err(); err != nil {
				t.Fatalf("enqueue: %v", err)
			}
			delivery, err := q.claim(context.Background())
			if err != nil || delivery == nil {
				t.Fatalf("claim = (%v, %v)", delivery, err)
			}
			if err := q.renew(context.Background(), delivery); err != nil {
				t.Fatalf("renew: %v", err)
			}
			if err := q.acknowledge(context.Background(), delivery); err != nil {
				t.Fatalf("ack: %v", err)
			}
			time.Sleep(110 * time.Millisecond)
			if err := q.acknowledge(context.Background(), delivery); err != nil {
				t.Fatalf("same-token ack confirmation: %v", err)
			}
		})
	}
}

func TestReliableLuaRedis7PrevalidationDoesNotLoseReadyTask(t *testing.T) {
	client := newLiveReliableClient(t)
	queue := fmt.Sprintf("gkit:103:prevalidate:{live-%d}", time.Now().UnixNano())
	defer cleanupReliableQueue(t, client, queue)
	q := newReliableQueue(client, queue, time.Second, newDeliveryTokenGenerator(nil))
	payload := []byte("prevalidate-task")
	if err := client.RPush(context.Background(), queue, payload).Err(); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	if err := client.Set(context.Background(), q.keys.inflight, "wrong-type", 0).Err(); err != nil {
		t.Fatalf("seed wrong type: %v", err)
	}
	if _, err := q.claim(context.Background()); err == nil {
		t.Fatal("claim error = nil, want key type failure")
	}
	if got := client.LIndex(context.Background(), queue, 0).Val(); got != string(payload) {
		t.Fatalf("ready task after prevalidation failure = %q, want %q", got, payload)
	}
}

func TestReliableLuaRedis7CommittedAckLostResponse(t *testing.T) {
	client := newLiveReliableClient(t)
	queue := fmt.Sprintf("gkit:103:lost-ack:{live-%d}", time.Now().UnixNano())
	defer cleanupReliableQueue(t, client, queue)
	q := newReliableQueue(client, queue, time.Second, newDeliveryTokenGenerator(nil))
	hook := &loseScriptResponseHook{hash: reliableAckScript.Hash(), err: errors.New("lost live ACK response")}
	client.AddHook(hook)
	if err := reliableAckScript.Load(context.Background(), client).Err(); err != nil {
		t.Fatalf("preload ACK script: %v", err)
	}
	if err := client.RPush(context.Background(), queue, "lost-response-task").Err(); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	delivery, err := q.claim(context.Background())
	if err != nil || delivery == nil {
		t.Fatalf("claim = (%v, %v)", delivery, err)
	}
	hook.armed.Store(true)
	c := &ControllerRedis{finalizationTimeout: 100 * time.Millisecond, ackConfirmationWindow: time.Second}
	if err := c.acknowledgeReliableDelivery(q, delivery); err != nil {
		t.Fatalf("confirm lost ACK response: %v", err)
	}
	if client.ZScore(context.Background(), q.keys.outcomes, delivery.token).Val() == 0 {
		t.Fatal("ACK outcome missing after lost response")
	}
}

func TestReliableRedisClusterQueueCompatibility(t *testing.T) {
	address := os.Getenv("GKIT_REDIS_CLUSTER_TEST_ADDR")
	if address == "" {
		t.Skip("GKIT_REDIS_CLUSTER_TEST_ADDR is not set")
	}
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: []string{address},
		ClusterSlots: func(context.Context) ([]redis.ClusterSlot, error) {
			return []redis.ClusterSlot{{Start: 0, End: 16383, Nodes: []redis.ClusterNode{{Addr: address}}}}, nil
		},
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		t.Fatalf("connect to Redis Cluster: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	for index, queue := range []string{
		fmt.Sprintf("gkit:103:cluster:%d:{tagged}", time.Now().UnixNano()),
		fmt.Sprintf("gkit:103:cluster:untagged:%d", time.Now().UnixNano()),
		fmt.Sprintf("gkit:103:cluster:{malformed:%d", time.Now().UnixNano()),
	} {
		t.Run(fmt.Sprintf("queue-%d", index), func(t *testing.T) {
			defer cleanupReliableQueue(t, client, queue)
			q := newReliableQueue(client, queue, time.Second, newDeliveryTokenGenerator(nil))
			if err := client.RPush(context.Background(), queue, "cluster-task").Err(); err != nil {
				t.Fatalf("enqueue: %v", err)
			}
			delivery, err := q.claim(context.Background())
			if err != nil || delivery == nil {
				t.Fatalf("cluster claim = (%v, %v)", delivery, err)
			}
			if err := q.acknowledge(context.Background(), delivery); err != nil {
				t.Fatalf("cluster ack: %v", err)
			}
		})
	}
}
