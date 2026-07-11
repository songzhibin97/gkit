package controller_redis

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
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
	if err := client.Del(ctx, queue, keys.inflight, keys.visibility, keys.outcomes, keys.repairCursor, keys.repairBacklog).Err(); err != nil {
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
	if err := client.Del(context.Background(), q.keys.inflight).Err(); err != nil {
		t.Fatalf("remove wrong-type inflight key: %v", err)
	}
	if err := client.HSet(context.Background(), q.keys.repairCursor, "wrong", "type").Err(); err != nil {
		t.Fatalf("seed wrong-type repair cursor: %v", err)
	}
	if _, err := q.claim(context.Background()); err == nil {
		t.Fatal("claim error = nil, want repair-cursor type failure")
	}
	if got := client.LIndex(context.Background(), queue, 0).Val(); got != string(payload) {
		t.Fatalf("ready task after cursor prevalidation failure = %q, want %q", got, payload)
	}
	if err := client.Del(context.Background(), q.keys.repairCursor).Err(); err != nil {
		t.Fatalf("remove wrong-type repair cursor: %v", err)
	}
	if err := client.HSet(context.Background(), q.keys.repairBacklog, "wrong", "type").Err(); err != nil {
		t.Fatalf("seed wrong type repair backlog: %v", err)
	}
	if _, err := q.claim(context.Background()); err == nil {
		t.Fatal("claim error = nil, want repair-backlog type failure")
	}
	if got := client.LIndex(context.Background(), queue, 0).Val(); got != string(payload) {
		t.Fatalf("ready task after backlog prevalidation failure = %q, want %q", got, payload)
	}
}

func TestReliableLuaRedis7ListpackTerminalPageContinuation(t *testing.T) {
	client := newLiveReliableClient(t)
	queue := fmt.Sprintf("gkit:103:listpack:{live-%d}", time.Now().UnixNano())
	defer cleanupReliableQueue(t, client, queue)
	q := newReliableQueue(client, queue, time.Minute, newDeliveryTokenGenerator(nil))
	const entries = 400
	deadline := float64(time.Now().Add(time.Hour).UnixMilli())
	ctx := context.Background()
	_, err := client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for index := 0; index < entries; index++ {
			token := fmt.Sprintf("listpack-token-%03d", index)
			payload := fmt.Sprintf("listpack-payload-%03d", index)
			pipe.HSet(ctx, q.keys.inflight, token, "0000000000:"+payload)
			pipe.ZAdd(ctx, q.keys.visibility, &redis.Z{Score: deadline, Member: token})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed listpack inflight hash: %v", err)
	}
	if encoding := client.Do(ctx, "OBJECT", "ENCODING", q.keys.inflight).Val(); encoding != "listpack" {
		t.Fatalf("inflight encoding = %v, want listpack", encoding)
	}
	page, next, err := client.HScan(ctx, q.keys.inflight, 0, "*", 128).Result()
	if err != nil {
		t.Fatalf("inspect listpack HSCAN page: %v", err)
	}
	if next != 0 || len(page)/2 != entries {
		t.Fatalf("listpack HSCAN = %d pairs, cursor %d; want %d pairs, cursor 0", len(page)/2, next, entries)
	}
	orphanToken := page[2*128]
	wantEnvelope := page[2*128+1]
	wantPayload := wantEnvelope[reliableEnvelopeHeaderSize:]
	if err := client.ZRem(ctx, q.keys.visibility, orphanToken).Err(); err != nil {
		t.Fatalf("make 129th listpack token orphan: %v", err)
	}

	if delivery, err := q.claim(ctx); err != nil || delivery != nil {
		t.Fatalf("first listpack claim = (%v, %v), want durable continuation only", delivery, err)
	}
	if score, err := client.ZScore(ctx, q.keys.repairBacklog, orphanToken).Result(); err != nil || score != 0 {
		t.Fatalf("129th orphan backlog score = %v, %v; want persisted score 0", score, err)
	}
	if cursor := client.Get(ctx, q.keys.repairCursor).Val(); cursor != "0" {
		t.Fatalf("terminal listpack cursor = %q, want 0", cursor)
	}
	if ttl := client.PTTL(ctx, q.keys.repairBacklog).Val(); ttl <= 0 || ttl > ackOutcomeKeyTTL {
		t.Fatalf("repair backlog TTL = %v, want (0, %v]", ttl, ackOutcomeKeyTTL)
	}

	var recovered *reliableDelivery
	for attempt := 0; attempt < 16 && recovered == nil; attempt++ {
		recovered, err = q.claim(ctx)
		if err != nil {
			t.Fatalf("drain listpack continuation %d: %v", attempt, err)
		}
	}
	if recovered == nil || string(recovered.payload) != wantPayload {
		t.Fatalf("recovered listpack delivery = %v, want payload %q", recovered, wantPayload)
	}
}

func TestReliableLuaRedis7RepairCallIsBounded(t *testing.T) {
	client := newLiveReliableClient(t)
	queue := fmt.Sprintf("gkit:103:bounded-repair:{live-%d}", time.Now().UnixNano())
	defer cleanupReliableQueue(t, client, queue)
	keys := deriveReliableQueueKeys(queue)
	const entries = 400
	ctx := context.Background()
	_, err := client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for index := 0; index < entries; index++ {
			token := fmt.Sprintf("bounded-token-%03d", index)
			pipe.HSet(ctx, keys.inflight, token, fmt.Sprintf("0000000000:bounded-payload-%03d", index))
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed bounded repair hash: %v", err)
	}
	result, err := reliableClaimScript.Run(ctx, client,
		[]string{queue, keys.inflight, keys.visibility, keys.outcomes, keys.repairCursor, keys.repairBacklog},
		"bounded-claim-token", time.Minute.Milliseconds(), ackOutcomeKeyTTL.Milliseconds()).Result()
	if err != nil {
		t.Fatalf("run one bounded repair call: %v", err)
	}
	values, err := reliableScriptValues(result)
	if err != nil {
		t.Fatal(err)
	}
	if code, err := reliableScriptInt(values[0]); err != nil || code != 3 {
		t.Fatalf("repair result = %v, %v; want code 3", values, err)
	}
	if ready := client.LLen(ctx, queue).Val(); ready != maxClaimReconciliations {
		t.Fatalf("repaired entries in one script = %d, want %d", ready, maxClaimReconciliations)
	}
	if remaining := client.HLen(ctx, keys.inflight).Val(); remaining != entries-maxClaimReconciliations {
		t.Fatalf("remaining inflight = %d, want %d", remaining, entries-maxClaimReconciliations)
	}
	if backlog := client.ZCard(ctx, keys.repairBacklog).Val(); backlog != entries-maxClaimReconciliations {
		t.Fatalf("durable continuation size = %d, want %d", backlog, entries-maxClaimReconciliations)
	}
}

func TestReliableLuaRedis7PersistentOrphanTraversal(t *testing.T) {
	client := newLiveReliableClient(t)
	queue := fmt.Sprintf("gkit:103:cursor:{live-%d}", time.Now().UnixNano())
	defer cleanupReliableQueue(t, client, queue)
	q := newReliableQueue(client, queue, time.Minute, newDeliveryTokenGenerator(nil))
	const entries = 2300
	deadline := float64(time.Now().Add(time.Hour).UnixMilli())
	ctx := context.Background()
	_, err := client.Pipelined(ctx, func(pipe redis.Pipeliner) error {
		for index := 0; index < entries; index++ {
			token := fmt.Sprintf("cursor-token-%04d", index)
			payload := fmt.Sprintf("cursor-payload-%04d", index)
			pipe.HSet(ctx, q.keys.inflight, token, "0000000000:"+payload)
			pipe.ZAdd(ctx, q.keys.visibility, &redis.Z{Score: deadline, Member: token})
		}
		return nil
	})
	if err != nil {
		t.Fatalf("seed stable inflight hash: %v", err)
	}
	firstPage, firstNext, err := client.HScan(ctx, q.keys.inflight, 0, "*", 128).Result()
	if err != nil {
		t.Fatalf("inspect first HSCAN page: %v", err)
	}
	if firstNext == 0 {
		t.Fatal("2300-entry hash unexpectedly fit in first HSCAN page")
	}
	firstTokens := make(map[string]struct{}, len(firstPage)/2)
	for index := 0; index < len(firstPage); index += 2 {
		firstTokens[firstPage[index]] = struct{}{}
	}
	var orphanToken string
	for index := entries - 1; index >= 0; index-- {
		candidate := fmt.Sprintf("cursor-token-%04d", index)
		if _, present := firstTokens[candidate]; !present {
			orphanToken = candidate
			break
		}
	}
	if orphanToken == "" {
		t.Fatal("could not choose orphan outside first HSCAN page")
	}
	wantPayload := "cursor-payload-" + orphanToken[len("cursor-token-"):]
	if err := client.ZRem(ctx, q.keys.visibility, orphanToken).Err(); err != nil {
		t.Fatalf("make later-page orphan: %v", err)
	}

	if delivery, err := q.claim(ctx); err != nil || delivery != nil {
		t.Fatalf("first-page claim = (%v, %v), want no recovered orphan", delivery, err)
	}
	if cursor := client.Get(ctx, q.keys.repairCursor).Val(); cursor == "" || cursor == "0" {
		t.Fatalf("persisted repair cursor after first page = %q, want nonzero", cursor)
	}
	if ttl := client.PTTL(ctx, q.keys.repairCursor).Val(); ttl <= 0 || ttl > ackOutcomeKeyTTL {
		t.Fatalf("repair cursor TTL = %v, want (0, %v]", ttl, ackOutcomeKeyTTL)
	}

	var recovered *reliableDelivery
	for attempt := 0; attempt < 256 && recovered == nil; attempt++ {
		recovered, err = q.claim(ctx)
		if err != nil {
			t.Fatalf("claim traversal attempt %d: %v", attempt, err)
		}
	}
	if recovered == nil || string(recovered.payload) != wantPayload {
		t.Fatalf("recovered delivery = %v, want payload %q", recovered, wantPayload)
	}

	for attempt := 0; attempt < 256 && (client.Get(ctx, q.keys.repairCursor).Val() != "0" || client.ZCard(ctx, q.keys.repairBacklog).Val() != 0); attempt++ {
		if _, err := q.claim(ctx); err != nil {
			t.Fatalf("finish stable cursor cycle %d: %v", attempt, err)
		}
	}
	if cursor := client.Get(ctx, q.keys.repairCursor).Val(); cursor != "0" {
		t.Fatalf("repair cursor did not complete stable cycle: %q", cursor)
	}
	if backlog := client.ZCard(ctx, q.keys.repairBacklog).Val(); backlog != 0 {
		t.Fatalf("repair backlog did not drain: %d", backlog)
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
	addresses := strings.Split(address, ",")
	if len(addresses) != 3 {
		t.Fatalf("GKIT_REDIS_CLUSTER_TEST_ADDR must list three master addresses, got %d", len(addresses))
	}
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs: addresses,
		ClusterSlots: func(context.Context) ([]redis.ClusterSlot, error) {
			return []redis.ClusterSlot{
				{Start: 0, End: 5460, Nodes: []redis.ClusterNode{{Addr: addresses[0]}}},
				{Start: 5461, End: 10922, Nodes: []redis.ClusterNode{{Addr: addresses[1]}}},
				{Start: 10923, End: 16383, Nodes: []redis.ClusterNode{{Addr: addresses[2]}}},
			}, nil
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
