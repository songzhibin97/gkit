package controller_redis

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

// These scripts are test-only copies of the write ordering in the production
// transitions. Production scripts contain no failpoint branches.
var claimFailpointScript = redis.NewScript(`
local payload = redis.call('LINDEX', KEYS[1], 0)
if not payload then return redis.error_reply('missing payload') end
local envelope = '0000000000:' .. payload
redis.call('HSET', KEYS[2], ARGV[1], envelope)
if tonumber(ARGV[3]) == 1 then return redis.error_reply('injected after HSET') end
redis.call('ZADD', KEYS[3], ARGV[2], ARGV[1])
if tonumber(ARGV[3]) == 2 then return redis.error_reply('injected after ZADD') end
redis.call('LPOP', KEYS[1])
if tonumber(ARGV[3]) == 3 then return redis.error_reply('injected after LPOP') end
return 1
`)

var moveFailpointScript = redis.NewScript(`
local envelope = redis.call('HGET', KEYS[1], ARGV[1])
if not envelope then return redis.error_reply('missing envelope') end
redis.call('HSET', KEYS[1], ARGV[2], envelope)
if tonumber(ARGV[4]) == 1 then return redis.error_reply('injected after destination HSET') end
redis.call('ZADD', KEYS[2], ARGV[3], ARGV[2])
if tonumber(ARGV[4]) == 2 then return redis.error_reply('injected after destination ZADD') end
redis.call('ZREM', KEYS[2], ARGV[1])
if tonumber(ARGV[4]) == 3 then return redis.error_reply('injected after source ZREM') end
redis.call('HDEL', KEYS[1], ARGV[1])
if tonumber(ARGV[4]) == 4 then return redis.error_reply('injected after source HDEL') end
return 1
`)

var releaseFailpointScript = redis.NewScript(`
local envelope = redis.call('HGET', KEYS[2], ARGV[1])
if not envelope then return redis.error_reply('missing envelope') end
local payload = string.sub(envelope, 12)
redis.call('RPUSH', KEYS[1], payload)
if tonumber(ARGV[2]) == 1 then return redis.error_reply('injected after RPUSH') end
redis.call('ZREM', KEYS[3], ARGV[1])
if tonumber(ARGV[2]) == 2 then return redis.error_reply('injected after ZREM') end
redis.call('HDEL', KEYS[2], ARGV[1])
if tonumber(ARGV[2]) == 3 then return redis.error_reply('injected after HDEL') end
return 1
`)

var ackFailpointScript = redis.NewScript(`
redis.call('ZADD', KEYS[3], ARGV[2], ARGV[1])
if tonumber(ARGV[4]) == 1 then return redis.error_reply('injected after outcome ZADD') end
redis.call('PEXPIRE', KEYS[3], ARGV[3])
if tonumber(ARGV[4]) == 2 then return redis.error_reply('injected after outcome PEXPIRE') end
redis.call('ZREM', KEYS[2], ARGV[1])
if tonumber(ARGV[4]) == 3 then return redis.error_reply('injected after visibility ZREM') end
redis.call('HDEL', KEYS[1], ARGV[1])
if tonumber(ARGV[4]) == 4 then return redis.error_reply('injected after inflight HDEL') end
return 1
`)

func TestLuaFailureBoundariesRetainRecoverableCopyOrAck(t *testing.T) {
	client := newLiveReliableClient(t)
	payload := []byte{0, 1, 2, 255, 't', 'a', 's', 'k'}

	t.Run("claim", func(t *testing.T) {
		for failpoint := 1; failpoint <= 3; failpoint++ {
			queue := fmt.Sprintf("gkit:103:failpoint:claim:%d:{live}", failpoint)
			keys := deriveReliableQueueKeys(queue)
			cleanupReliableQueue(t, client, queue)
			if err := client.RPush(context.Background(), queue, payload).Err(); err != nil {
				t.Fatalf("seed failpoint %d: %v", failpoint, err)
			}
			_, err := claimFailpointScript.Run(context.Background(), client,
				[]string{queue, keys.inflight, keys.visibility}, "claim-token", time.Now().Add(time.Minute).UnixMilli(), failpoint).Result()
			if err == nil {
				t.Fatalf("failpoint %d returned nil error", failpoint)
			}
			assertRecoverablePayload(t, client, keys, payload)
			cleanupReliableQueue(t, client, queue)
		}
	})

	for _, transition := range []string{"reclaim", "retry"} {
		t.Run(transition, func(t *testing.T) {
			for failpoint := 1; failpoint <= 4; failpoint++ {
				queue := fmt.Sprintf("gkit:103:failpoint:%s:%d:{live}", transition, failpoint)
				keys := deriveReliableQueueKeys(queue)
				cleanupReliableQueue(t, client, queue)
				seedReservedPayload(t, client, keys, "old-token", payload)
				_, err := moveFailpointScript.Run(context.Background(), client,
					[]string{keys.inflight, keys.visibility}, "old-token", "new-token",
					time.Now().Add(time.Minute).UnixMilli(), failpoint).Result()
				if err == nil {
					t.Fatalf("failpoint %d returned nil error", failpoint)
				}
				assertRecoverablePayload(t, client, keys, payload)
				cleanupReliableQueue(t, client, queue)
			}
		})
	}

	t.Run("release", func(t *testing.T) {
		for failpoint := 1; failpoint <= 3; failpoint++ {
			queue := fmt.Sprintf("gkit:103:failpoint:release:%d:{live}", failpoint)
			keys := deriveReliableQueueKeys(queue)
			cleanupReliableQueue(t, client, queue)
			seedReservedPayload(t, client, keys, "release-token", payload)
			_, err := releaseFailpointScript.Run(context.Background(), client,
				[]string{queue, keys.inflight, keys.visibility}, "release-token", failpoint).Result()
			if err == nil {
				t.Fatalf("failpoint %d returned nil error", failpoint)
			}
			assertRecoverablePayload(t, client, keys, payload)
			cleanupReliableQueue(t, client, queue)
		}
	})

	t.Run("ack", func(t *testing.T) {
		for failpoint := 1; failpoint <= 4; failpoint++ {
			queue := fmt.Sprintf("gkit:103:failpoint:ack:%d:{live}", failpoint)
			keys := deriveReliableQueueKeys(queue)
			cleanupReliableQueue(t, client, queue)
			seedReservedPayload(t, client, keys, "ack-token", payload)
			outcome := time.Now().Add(ackOutcomeRetention).UnixMilli()
			_, err := ackFailpointScript.Run(context.Background(), client,
				[]string{keys.inflight, keys.visibility, keys.outcomes}, "ack-token", outcome,
				ackOutcomeKeyTTL.Milliseconds(), failpoint).Result()
			if err == nil {
				t.Fatalf("failpoint %d returned nil error", failpoint)
			}
			if score := client.ZScore(context.Background(), keys.outcomes, "ack-token").Val(); score == 0 {
				t.Fatalf("failpoint %d left no ACK outcome", failpoint)
			}
			q := newReliableQueue(client, queue, time.Minute, newDeliveryTokenGenerator(nil))
			delivery := &reliableDelivery{token: "ack-token", payload: payload}
			if err := q.acknowledge(context.Background(), delivery); err != nil {
				t.Fatalf("failpoint %d same-token confirmation: %v", failpoint, err)
			}
			cleanupReliableQueue(t, client, queue)
		}
	})
}

func seedReservedPayload(t *testing.T, client redis.UniversalClient, keys reliableQueueKeys, token string, payload []byte) {
	t.Helper()
	envelope := append([]byte("0000000000:"), payload...)
	if err := client.HSet(context.Background(), keys.inflight, token, envelope).Err(); err != nil {
		t.Fatalf("seed inflight: %v", err)
	}
	if err := client.ZAdd(context.Background(), keys.visibility, &redis.Z{
		Score:  float64(time.Now().Add(time.Minute).UnixMilli()),
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
