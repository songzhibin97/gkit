package controller_redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/go-redis/redis/v8"
)

// Delayed tasks move from the delayed zset to their target ready queue in two
// steps: an atomic Lua claim moves the due member into a same-slot transit
// zset (scored by Redis server time in milliseconds), then the producer
// decodes and republishes it from Go. A crash between the two steps therefore
// leaves the member in transit instead of losing it; recoverDelayedTransit
// moves members whose claim is older than delayedTransitRecoveryTimeout back
// to the delayed zset for a fresh claim. Delivery is at-least-once: a
// publisher slower than the recovery timeout can race recovery and deliver
// twice, but a claimed task is never lost.
//
// The at-least-once invariant holds without a per-claim token because every
// transition that removes a member is either atomic or publish-backed: the
// claim, restore, and recover scripts move the member between the delayed and
// transit zsets in a single atomic step (ZADD before ZREM), and
// finalizeDelayedTransit only ZREMs a transit member after its Publish (RPush
// to the ready queue) has succeeded. Members are keyed by task body, so a
// slow publisher racing recovery and a second claimant can finalize away the
// other claimant's transit entry — but only after at least one publish
// landed, and a restore racing a finalize can at worst re-schedule an
// already-published body. Every interleaving therefore degrades to duplicate
// delivery, never to loss, which is why no claim token is needed.

const (
	// delayedTransitRecoveryTimeout is how long a claimed delayed task may sit
	// in the transit zset before recovery treats its publisher as crashed. It
	// is also the default for ControllerRedis.delayedRecoveryInterval, which
	// paces the periodic recovery scan in produceDelayedTasks.
	delayedTransitRecoveryTimeout = 30 * time.Second
	// delayedTransitRecoveryBatch mirrors the bounded housekeeping batches
	// used by the reliable queue scripts.
	delayedTransitRecoveryBatch = 128
)

// deriveDelayedTransitKey returns the staging key that holds claimed delayed
// tasks while they are republished. It shares the delayed queue's Redis
// Cluster slot so the claim/restore/recover scripts stay single-slot.
func deriveDelayedTransitKey(queue string) string {
	return deriveReliableQueueKeys(queue).prefix + ":delayed-transit"
}

// delayedClaimScript atomically moves the earliest due member (FIFO by ETA
// score) from the delayed zset (KEYS[1]) into the transit zset (KEYS[2]),
// scored with Redis server time. The destination write precedes the
// destructive source removal, and the script is atomic, so no client crash
// can leave the member in neither key. Returns {member, score} or false when
// nothing is due before ARGV[1].
const delayedClaimScriptSource = `
local function typename(key)
  local value = redis.call('TYPE', key)
  if type(value) == 'table' then return value['ok'] end
  return value
end
local function requiretype(key, expected)
  local actual = typename(key)
  return actual == 'none' or actual == expected
end
local function nowms()
  local now = redis.call('TIME')
  return tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
end
if not requiretype(KEYS[1], 'zset') or not requiretype(KEYS[2], 'zset') then
  return redis.error_reply('delayed claim: invalid key type')
end
local max = ARGV[1]
if not max or not tonumber(max) then
  return redis.error_reply('delayed claim: invalid argument')
end
local due = redis.call('ZRANGEBYSCORE', KEYS[1], '0', max, 'WITHSCORES', 'LIMIT', 0, 1)
if #due ~= 2 then return false end
redis.call('ZADD', KEYS[2], nowms(), due[1])
redis.call('ZREM', KEYS[1], due[1])
return {due[1], due[2]}
`

var delayedClaimScript = redis.NewScript(delayedClaimScriptSource)

// delayedRestoreScript atomically puts a claimed member back into the delayed
// zset (KEYS[1]) with its original ETA score and clears its transit entry
// (KEYS[2]). Used when republishing fails without a crash.
const delayedRestoreScriptSource = `
local function typename(key)
  local value = redis.call('TYPE', key)
  if type(value) == 'table' then return value['ok'] end
  return value
end
local function requiretype(key, expected)
  local actual = typename(key)
  return actual == 'none' or actual == expected
end
if not requiretype(KEYS[1], 'zset') or not requiretype(KEYS[2], 'zset') then
  return redis.error_reply('delayed restore: invalid key type')
end
if not ARGV[1] or ARGV[1] == '' or not ARGV[2] or not tonumber(ARGV[2]) then
  return redis.error_reply('delayed restore: invalid argument')
end
redis.call('ZADD', KEYS[1], ARGV[2], ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
return 1
`

var delayedRestoreScript = redis.NewScript(delayedRestoreScriptSource)

// delayedRecoverScript moves transit members (KEYS[2]) whose claim is older
// than ARGV[1] milliseconds of Redis server time back into the delayed zset
// (KEYS[1]), keeping their claim score so recovered members stay due
// immediately and preserve claim order. Bounded to one batch per call;
// returns the number of members moved.
const delayedRecoverScriptSource = `
local function typename(key)
  local value = redis.call('TYPE', key)
  if type(value) == 'table' then return value['ok'] end
  return value
end
local function requiretype(key, expected)
  local actual = typename(key)
  return actual == 'none' or actual == expected
end
local function nowms()
  local now = redis.call('TIME')
  return tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
end
if not requiretype(KEYS[1], 'zset') or not requiretype(KEYS[2], 'zset') then
  return redis.error_reply('delayed recover: invalid key type')
end
local timeout = tonumber(ARGV[1])
if not timeout or timeout <= 0 then
  return redis.error_reply('delayed recover: invalid argument')
end
local stale = redis.call('ZRANGEBYSCORE', KEYS[2], '-inf', nowms() - timeout, 'WITHSCORES', 'LIMIT', 0, 128)
if #stale == 0 then return 0 end
local members = {}
for index = 1, #stale, 2 do
  redis.call('ZADD', KEYS[1], stale[index + 1], stale[index])
  members[#members + 1] = stale[index]
end
redis.call('ZREM', KEYS[2], unpack(members))
return #members
`

var delayedRecoverScript = redis.NewScript(delayedRecoverScriptSource)

// claimDelayedTask atomically claims the earliest due delayed task into the
// transit zset and returns its body and original ETA score. Returns redis.Nil
// (from the script's nil reply) when nothing is due.
func (c *ControllerRedis) claimDelayedTask(ctx context.Context, queue string) ([]byte, float64, error) {
	max := strconv.FormatInt(time.Now().Local().UnixNano(), 10)
	result, err := delayedClaimScript.Run(ctx, c.client, []string{queue, deriveDelayedTransitKey(queue)}, max).Result()
	if err != nil {
		return nil, 0, err
	}
	values, err := reliableScriptValues(result)
	if err != nil {
		return nil, 0, err
	}
	if len(values) != 2 {
		return nil, 0, fmt.Errorf("claim delayed task: invalid script response")
	}
	member, err := reliableScriptBytes(values[0])
	if err != nil {
		return nil, 0, err
	}
	scoreBytes, err := reliableScriptBytes(values[1])
	if err != nil {
		return nil, 0, err
	}
	score, err := strconv.ParseFloat(string(scoreBytes), 64)
	if err != nil {
		return nil, 0, fmt.Errorf("claim delayed task: invalid score: %w", err)
	}
	return member, score, nil
}

// finalizeDelayedTransit clears a claimed member from the transit zset after
// its republish succeeded. Uses a background context like the restore paths
// so a consume-attempt cancellation cannot strand a published task in transit
// (which would make recovery deliver it a second time on every shutdown).
func (c *ControllerRedis) finalizeDelayedTransit(queue string, taskBody []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), consumerRestoreTimeout)
	defer cancel()
	if err := c.client.ZRem(ctx, deriveDelayedTransitKey(queue), taskBody).Err(); err != nil {
		return wrapRedisOperation("finalize delayed task", err)
	}
	return nil
}

// recoverDelayedTransit drains every stale transit member back into the
// delayed zset, one bounded batch at a time.
func (c *ControllerRedis) recoverDelayedTransit(ctx context.Context, queue string) error {
	keys := []string{queue, deriveDelayedTransitKey(queue)}
	for {
		result, err := delayedRecoverScript.Run(ctx, c.client, keys, delayedTransitRecoveryTimeout.Milliseconds()).Result()
		if err != nil {
			return wrapRedisOperation("recover delayed tasks", err)
		}
		moved, err := reliableScriptInt(result)
		if err != nil {
			return err
		}
		if moved < delayedTransitRecoveryBatch {
			return nil
		}
	}
}
