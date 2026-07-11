package controller_redis

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

type productionLuaBoundary struct {
	name       string
	command    string
	occurrence int
}

func injectProductionLuaFailure(t *testing.T, source string, boundary productionLuaBoundary) *redis.Script {
	t.Helper()
	if boundary.occurrence < 1 {
		t.Fatalf("boundary %q occurrence = %d", boundary.name, boundary.occurrence)
	}
	searchFrom := 0
	commandStart := -1
	for occurrence := 0; occurrence < boundary.occurrence; occurrence++ {
		relative := strings.Index(source[searchFrom:], boundary.command)
		if relative < 0 {
			t.Fatalf("production boundary %q occurrence %d not found", boundary.name, boundary.occurrence)
		}
		commandStart = searchFrom + relative
		searchFrom = commandStart + len(boundary.command)
	}
	commandEnd := commandStart + len(boundary.command)
	injected := source[:commandEnd] +
		"\nif true then return redis.error_reply('injected production boundary: " + boundary.name + "') end" +
		source[commandEnd:]
	return redis.NewScript(injected)
}

func TestLuaFailureBoundariesRetainRecoverableCopyOrAck(t *testing.T) {
	client := newLiveReliableClient(t)
	payload := []byte{0, 1, 2, 255, 't', 'a', 's', 'k'}

	t.Run("claim", func(t *testing.T) {
		boundaries := []productionLuaBoundary{
			{name: "repair cursor SET", command: "redis.call('SET', KEYS[5], next_cursor, 'PX', outcome_ttl)", occurrence: 1},
			{name: "destination HSET", command: "redis.call('HSET', KEYS[2], token, envelope)", occurrence: 1},
			{name: "destination ZADD", command: "redis.call('ZADD', KEYS[3], deadline, token)", occurrence: 2},
			{name: "source LPOP", command: "redis.call('LPOP', KEYS[1])", occurrence: 1},
		}
		for index, boundary := range boundaries {
			queue := fmt.Sprintf("gkit:103:production:claim:%d:{live}", index)
			keys := deriveReliableQueueKeys(queue)
			cleanupReliableQueue(t, client, queue)
			if err := client.RPush(context.Background(), queue, payload).Err(); err != nil {
				t.Fatalf("seed %s: %v", boundary.name, err)
			}
			script := injectProductionLuaFailure(t, reliableClaimScriptSource, boundary)
			_, err := script.Run(context.Background(), client,
				[]string{queue, keys.inflight, keys.visibility, keys.outcomes, keys.repairCursor},
				"claim-token", time.Minute.Milliseconds(), ackOutcomeKeyTTL.Milliseconds()).Result()
			if err == nil {
				t.Fatalf("boundary %q returned nil error", boundary.name)
			}
			assertRecoverablePayload(t, client, keys, payload)
			cleanupReliableQueue(t, client, queue)
		}
	})

	t.Run("reclaim", func(t *testing.T) {
		boundaries := []productionLuaBoundary{
			{name: "repair cursor SET", command: "redis.call('SET', KEYS[5], next_cursor, 'PX', outcome_ttl)", occurrence: 1},
			{name: "destination HSET", command: "redis.call('HSET', KEYS[2], token, oldenvelope)", occurrence: 1},
			{name: "destination ZADD", command: "redis.call('ZADD', KEYS[3], deadline, token)", occurrence: 1},
			{name: "source ZREM", command: "redis.call('ZREM', KEYS[3], oldtoken)", occurrence: 3},
			{name: "source HDEL", command: "redis.call('HDEL', KEYS[2], oldtoken)", occurrence: 2},
		}
		for index, boundary := range boundaries {
			queue := fmt.Sprintf("gkit:103:production:reclaim:%d:{live}", index)
			keys := deriveReliableQueueKeys(queue)
			cleanupReliableQueue(t, client, queue)
			seedReservedPayloadAt(t, client, keys, "old-token", payload, 0)
			script := injectProductionLuaFailure(t, reliableClaimScriptSource, boundary)
			_, err := script.Run(context.Background(), client,
				[]string{queue, keys.inflight, keys.visibility, keys.outcomes, keys.repairCursor},
				"new-token", time.Minute.Milliseconds(), ackOutcomeKeyTTL.Milliseconds()).Result()
			if err == nil {
				t.Fatalf("boundary %q returned nil error", boundary.name)
			}
			assertRecoverablePayload(t, client, keys, payload)
			cleanupReliableQueue(t, client, queue)
		}
	})

	t.Run("deferred retry", func(t *testing.T) {
		boundaries := []productionLuaBoundary{
			{name: "destination HSET", command: "redis.call('HSET', KEYS[1], newtoken, newenvelope)", occurrence: 1},
			{name: "destination ZADD", command: "redis.call('ZADD', KEYS[2], deadline, newtoken)", occurrence: 1},
			{name: "source ZREM", command: "redis.call('ZREM', KEYS[2], oldtoken)", occurrence: 1},
			{name: "source HDEL", command: "redis.call('HDEL', KEYS[1], oldtoken)", occurrence: 1},
		}
		for index, boundary := range boundaries {
			queue := fmt.Sprintf("gkit:103:production:retry:%d:{live}", index)
			keys := deriveReliableQueueKeys(queue)
			cleanupReliableQueue(t, client, queue)
			seedReservedPayloadAt(t, client, keys, "old-token", payload, time.Now().Add(time.Minute).UnixMilli())
			script := injectProductionLuaFailure(t, reliableDeferScriptSource, boundary)
			_, err := script.Run(context.Background(), client,
				[]string{keys.inflight, keys.visibility, keys.outcomes},
				"old-token", "new-token", time.Second.Milliseconds()).Result()
			if err == nil {
				t.Fatalf("boundary %q returned nil error", boundary.name)
			}
			assertRecoverablePayload(t, client, keys, payload)
			cleanupReliableQueue(t, client, queue)
		}
	})

	t.Run("release", func(t *testing.T) {
		boundaries := []productionLuaBoundary{
			{name: "destination RPUSH", command: "redis.call('RPUSH', KEYS[1], payload)", occurrence: 1},
			{name: "source ZREM", command: "redis.call('ZREM', KEYS[3], token)", occurrence: 1},
			{name: "source HDEL", command: "redis.call('HDEL', KEYS[2], token)", occurrence: 1},
		}
		for index, boundary := range boundaries {
			queue := fmt.Sprintf("gkit:103:production:release:%d:{live}", index)
			keys := deriveReliableQueueKeys(queue)
			cleanupReliableQueue(t, client, queue)
			seedReservedPayloadAt(t, client, keys, "release-token", payload, time.Now().Add(time.Minute).UnixMilli())
			script := injectProductionLuaFailure(t, reliableReleaseScriptSource, boundary)
			_, err := script.Run(context.Background(), client,
				[]string{queue, keys.inflight, keys.visibility}, "release-token").Result()
			if err == nil {
				t.Fatalf("boundary %q returned nil error", boundary.name)
			}
			assertRecoverablePayload(t, client, keys, payload)
			cleanupReliableQueue(t, client, queue)
		}
	})

	t.Run("ack", func(t *testing.T) {
		boundaries := []productionLuaBoundary{
			{name: "outcome ZADD", command: "redis.call('ZADD', KEYS[3], outcome, token)", occurrence: 1},
			{name: "outcome PEXPIRE", command: "redis.call('PEXPIRE', KEYS[3], ttl)", occurrence: 2},
			{name: "visibility ZREM", command: "redis.call('ZREM', KEYS[2], token)", occurrence: 2},
			{name: "inflight HDEL", command: "redis.call('HDEL', KEYS[1], token)", occurrence: 2},
		}
		for index, boundary := range boundaries {
			queue := fmt.Sprintf("gkit:103:production:ack:%d:{live}", index)
			keys := deriveReliableQueueKeys(queue)
			cleanupReliableQueue(t, client, queue)
			seedReservedPayloadAt(t, client, keys, "ack-token", payload, time.Now().Add(time.Minute).UnixMilli())
			script := injectProductionLuaFailure(t, reliableAckScriptSource, boundary)
			_, err := script.Run(context.Background(), client,
				[]string{keys.inflight, keys.visibility, keys.outcomes}, "ack-token",
				ackOutcomeRetention.Milliseconds(), ackOutcomeKeyTTL.Milliseconds()).Result()
			if err == nil {
				t.Fatalf("boundary %q returned nil error", boundary.name)
			}
			if score := client.ZScore(context.Background(), keys.outcomes, "ack-token").Val(); score == 0 {
				t.Fatalf("boundary %q left no ACK outcome", boundary.name)
			}
			q := newReliableQueue(client, queue, time.Minute, newDeliveryTokenGenerator(nil))
			if err := q.acknowledge(context.Background(), &reliableDelivery{token: "ack-token", payload: payload}); err != nil {
				t.Fatalf("boundary %q same-token confirmation: %v", boundary.name, err)
			}
			cleanupReliableQueue(t, client, queue)
		}
	})
}

func seedReservedPayloadAt(
	t *testing.T,
	client redis.UniversalClient,
	keys reliableQueueKeys,
	token string,
	payload []byte,
	deadlineMillis int64,
) {
	t.Helper()
	envelope := append([]byte("0000000000:"), payload...)
	if err := client.HSet(context.Background(), keys.inflight, token, envelope).Err(); err != nil {
		t.Fatalf("seed inflight: %v", err)
	}
	if err := client.ZAdd(context.Background(), keys.visibility, &redis.Z{
		Score:  float64(deadlineMillis),
		Member: token,
	}).Err(); err != nil {
		t.Fatalf("seed visibility: %v", err)
	}
}

func assertRecoverablePayload(t *testing.T, client redis.UniversalClient, keys reliableQueueKeys, payload []byte) {
	t.Helper()
	ready, err := client.LRange(context.Background(), keys.ready, 0, -1).Result()
	if err != nil && err != redis.Nil {
		t.Fatalf("read ready: %v", err)
	}
	for _, candidate := range ready {
		if bytes.Equal([]byte(candidate), payload) {
			return
		}
	}
	inflight, err := client.HVals(context.Background(), keys.inflight).Result()
	if err != nil && err != redis.Nil {
		t.Fatalf("read inflight: %v", err)
	}
	for _, envelope := range inflight {
		if len(envelope) >= reliableEnvelopeHeaderSize && bytes.Equal([]byte(envelope[reliableEnvelopeHeaderSize:]), payload) {
			return
		}
	}
	t.Fatal("transition failure left no recoverable payload")
}
