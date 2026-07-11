package controller_redis

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	ErrDeliveryLeaseLost        = errors.New("redis delivery lease lost")
	ErrDeliveryTokenUnavailable = errors.New("redis delivery token unavailable")
	ErrDeliveryTokenCollision   = errors.New("redis delivery token collision limit reached")
)

const (
	defaultDeliveryLease       = 30 * time.Second
	ackOutcomeRetention        = 24 * time.Hour
	ackOutcomeKeyTTL           = 25 * time.Hour
	maxDeliveryTokenAttempts   = 4
	maxClaimReconciliations    = 128
	reliableEnvelopeHeaderSize = 11
)

type deliveryTokenGenerator struct {
	mu     sync.Mutex
	reader io.Reader
}

func newDeliveryTokenGenerator(reader io.Reader) *deliveryTokenGenerator {
	if reader == nil {
		reader = rand.Reader
	}
	return &deliveryTokenGenerator{reader: reader}
}

func (g *deliveryTokenGenerator) next() (string, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	var token [16]byte
	if _, err := io.ReadFull(g.reader, token[:]); err != nil {
		return "", fmt.Errorf("%w: %w", ErrDeliveryTokenUnavailable, err)
	}
	return hex.EncodeToString(token[:]), nil
}

type reliableDelivery struct {
	token          string
	payload        []byte
	failures       uint64
	serverTime     time.Time
	deadline       time.Time
	confirmedUntil time.Time
}

func (d *reliableDelivery) updateConfirmation(requestStarted time.Time, serverMillis, deadlineMillis int64) {
	d.serverTime = time.UnixMilli(serverMillis)
	d.deadline = time.UnixMilli(deadlineMillis)
	remaining := time.Duration(deadlineMillis-serverMillis) * time.Millisecond
	if remaining < 0 {
		remaining = 0
	}
	d.confirmedUntil = requestStarted.Add(remaining)
}

type reliableQueue struct {
	client      redis.UniversalClient
	keys        reliableQueueKeys
	lease       time.Duration
	tokenSource *deliveryTokenGenerator
}

func newReliableQueue(client redis.UniversalClient, queue string, lease time.Duration, tokenSource *deliveryTokenGenerator) *reliableQueue {
	if lease <= 0 {
		lease = defaultDeliveryLease
	}
	if tokenSource == nil {
		tokenSource = newDeliveryTokenGenerator(nil)
	}
	return &reliableQueue{
		client:      client,
		keys:        deriveReliableQueueKeys(queue),
		lease:       lease,
		tokenSource: tokenSource,
	}
}

const reliableClaimScriptSource = `
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
if not requiretype(KEYS[1], 'list') or not requiretype(KEYS[2], 'hash') or
   not requiretype(KEYS[3], 'zset') or not requiretype(KEYS[4], 'zset') or
   not requiretype(KEYS[5], 'string') or not requiretype(KEYS[6], 'zset') then
  return redis.error_reply('reliable claim: invalid key type')
end
local token = ARGV[1]
local lease = tonumber(ARGV[2])
local outcome_ttl = tonumber(ARGV[3])
if not token or token == '' or not lease or lease <= 0 or not outcome_ttl or outcome_ttl <= 0 then
  return redis.error_reply('reliable claim: invalid argument')
end
if redis.call('HEXISTS', KEYS[2], token) == 1 or redis.call('ZSCORE', KEYS[3], token) or
   redis.call('ZSCORE', KEYS[4], token) or redis.call('ZSCORE', KEYS[6], token) then
  return {2}
end
local now = nowms()
local repair_cursor = redis.call('GET', KEYS[5]) or '0'
if not string.match(repair_cursor, '^%d+$') then
  return redis.error_reply('reliable claim: invalid repair cursor')
end
local expired = redis.call('ZRANGEBYSCORE', KEYS[4], '-inf', now, 'LIMIT', 0, 128)
local outcome_count = redis.call('ZCARD', KEYS[4])
local outcome_ttl_missing = outcome_count > 0 and redis.call('PTTL', KEYS[4]) < 0
local newest = redis.call('ZREVRANGE', KEYS[4], 0, 0, 'WITHSCORES')
local backlog_tokens = redis.call('ZRANGE', KEYS[6], 0, 127)
local draining_backlog = #backlog_tokens > 0
local next_cursor = repair_cursor
local overflow_tokens = {}
local repair = {}
if draining_backlog then
  for index = 1, #backlog_tokens do
    local repair_token = backlog_tokens[index]
    repair[#repair + 1] = {
      token = repair_token,
      envelope = redis.call('HGET', KEYS[2], repair_token),
      outcome = redis.call('ZSCORE', KEYS[4], repair_token),
      visibility = redis.call('ZSCORE', KEYS[3], repair_token)
    }
  end
else
  local scan = redis.call('HSCAN', KEYS[2], repair_cursor, 'COUNT', 128)
  local entries = scan[2]
  next_cursor = scan[1]
  if #entries % 2 ~= 0 then
    return redis.error_reply('reliable claim: invalid repair scan page')
  end
  local entry_limit = math.min(#entries, 256)
  for index = 1, entry_limit, 2 do
    local repair_token = entries[index]
    repair[#repair + 1] = {
      token = repair_token,
      envelope = redis.call('HGET', KEYS[2], repair_token),
      outcome = redis.call('ZSCORE', KEYS[4], repair_token),
      visibility = redis.call('ZSCORE', KEYS[3], repair_token)
    }
  end
  for index = entry_limit + 1, #entries, 2 do
    overflow_tokens[#overflow_tokens + 1] = entries[index]
  end
end
for index = 1, #repair do
  local envelope = repair[index].envelope
  if envelope and (string.len(envelope) < 11 or string.sub(envelope, 11, 11) ~= ':' or
     not tonumber(string.sub(envelope, 1, 10))) then
    return redis.error_reply('reliable claim: invalid orphan envelope')
  end
end
local entry_count = #repair
local visible = {}
local visibility_budget = 128 - entry_count
if visibility_budget > 0 then
  visible = redis.call('ZRANGE', KEYS[3], 0, visibility_budget - 1)
end
local due = redis.call('ZRANGEBYSCORE', KEYS[3], '-inf', now, 'LIMIT', 0, 1)
local oldtoken = nil
local oldoutcome = nil
local oldenvelope = nil
if #due == 1 then
  oldtoken = due[1]
  oldoutcome = redis.call('ZSCORE', KEYS[4], oldtoken)
  oldenvelope = redis.call('HGET', KEYS[2], oldtoken)
  if oldenvelope and (string.len(oldenvelope) < 11 or string.sub(oldenvelope, 11, 11) ~= ':' or
     not tonumber(string.sub(oldenvelope, 1, 10))) then
    return redis.error_reply('reliable claim: invalid envelope')
  end
end
local payload = redis.call('LINDEX', KEYS[1], 0)

-- Every validation above precedes the first write. Housekeeping is bounded
-- and returns for a fresh claim if it changed recoverable state.
local reconciled = false
if #expired > 0 then
  redis.call('ZREM', KEYS[4], unpack(expired))
  reconciled = true
end
if outcome_ttl_missing then
  local repaired_ttl = outcome_ttl
  if #newest == 2 then repaired_ttl = math.max(1, tonumber(newest[2]) - now + 3600000) end
  redis.call('PEXPIRE', KEYS[4], repaired_ttl)
end
if not draining_backlog then
  if #overflow_tokens > 0 then
    local backlog_add = {}
    for index = 1, #overflow_tokens do
      backlog_add[#backlog_add + 1] = 0
      backlog_add[#backlog_add + 1] = overflow_tokens[index]
    end
    redis.call('ZADD', KEYS[6], 'NX', unpack(backlog_add))
  end
else
  redis.call('PEXPIRE', KEYS[5], outcome_ttl)
end
for index = 1, #repair do
  local orphan_token = repair[index].token
  local envelope = repair[index].envelope
  local outcome = repair[index].outcome
  local visibility = repair[index].visibility
  if not envelope then
    if visibility then
      redis.call('ZREM', KEYS[3], orphan_token)
      reconciled = true
    end
  elseif outcome and tonumber(outcome) > now then
    redis.call('ZREM', KEYS[3], orphan_token)
    redis.call('HDEL', KEYS[2], orphan_token)
    reconciled = true
  elseif not visibility then
    redis.call('RPUSH', KEYS[1], string.sub(envelope, 12))
    redis.call('HDEL', KEYS[2], orphan_token)
    reconciled = true
  end
end
if draining_backlog then redis.call('ZREM', KEYS[6], unpack(backlog_tokens)) end
for index = 1, #visible do
  local visible_token = visible[index]
  if redis.call('HEXISTS', KEYS[2], visible_token) == 0 then
    redis.call('ZREM', KEYS[3], visible_token)
    reconciled = true
  end
end
if not draining_backlog then redis.call('SET', KEYS[5], next_cursor, 'PX', outcome_ttl) end
if redis.call('ZCARD', KEYS[6]) > 0 then redis.call('PEXPIRE', KEYS[6], outcome_ttl) end
if reconciled then return {3} end

if oldtoken then
  if oldoutcome and tonumber(oldoutcome) > now then
    redis.call('ZREM', KEYS[3], oldtoken)
    redis.call('HDEL', KEYS[2], oldtoken)
    return {3}
  end
  if not oldenvelope then
    redis.call('ZREM', KEYS[3], oldtoken)
    return {3}
  end
  local deadline = now + lease
  redis.call('HSET', KEYS[2], token, oldenvelope)
  redis.call('ZADD', KEYS[3], deadline, token)
  redis.call('ZREM', KEYS[3], oldtoken)
  redis.call('HDEL', KEYS[2], oldtoken)
  return {1, token, oldenvelope, tostring(now), tostring(deadline)}
end
if not payload then return {0, tostring(now)} end
local envelope = '0000000000:' .. payload
local deadline = now + lease
redis.call('HSET', KEYS[2], token, envelope)
redis.call('ZADD', KEYS[3], deadline, token)
local removed = redis.call('LPOP', KEYS[1])
if not removed or removed ~= payload then
  return redis.error_reply('reliable claim: ready payload changed')
end
return {1, token, envelope, tostring(now), tostring(deadline)}
`

var reliableClaimScript = redis.NewScript(reliableClaimScriptSource)

const reliableRenewScriptSource = `
local function typename(key)
  local value = redis.call('TYPE', key)
  if type(value) == 'table' then return value['ok'] end
  return value
end
local function nowms()
  local now = redis.call('TIME')
  return tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
end
local ht = typename(KEYS[1])
local zt = typename(KEYS[2])
if (ht ~= 'none' and ht ~= 'hash') or (zt ~= 'none' and zt ~= 'zset') then
  return redis.error_reply('reliable renew: invalid key type')
end
local token = ARGV[1]
local lease = tonumber(ARGV[2])
if not token or token == '' or not lease or lease <= 0 then
  return redis.error_reply('reliable renew: invalid argument')
end
local now = nowms()
local envelope = redis.call('HGET', KEYS[1], token)
local score = redis.call('ZSCORE', KEYS[2], token)
if not envelope or not score or tonumber(score) <= now then return {0, tostring(now)} end
local deadline = now + lease
redis.call('ZADD', KEYS[2], deadline, token)
return {1, tostring(now), tostring(deadline)}
`

var reliableRenewScript = redis.NewScript(reliableRenewScriptSource)

const reliableAckScriptSource = `
local function typename(key)
  local value = redis.call('TYPE', key)
  if type(value) == 'table' then return value['ok'] end
  return value
end
local function nowms()
  local now = redis.call('TIME')
  return tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
end
local ht = typename(KEYS[1])
local vt = typename(KEYS[2])
local ot = typename(KEYS[3])
local bt = typename(KEYS[4])
if (ht ~= 'none' and ht ~= 'hash') or (vt ~= 'none' and vt ~= 'zset') or
   (ot ~= 'none' and ot ~= 'zset') or (bt ~= 'none' and bt ~= 'zset') then
  return redis.error_reply('reliable ack: invalid key type')
end
local token = ARGV[1]
local retention = tonumber(ARGV[2])
local ttl = tonumber(ARGV[3])
if not token or token == '' or not retention or retention <= 0 or not ttl or ttl <= retention then
  return redis.error_reply('reliable ack: invalid argument')
end
local now = nowms()
local prior = redis.call('ZSCORE', KEYS[3], token)
local expired = redis.call('ZRANGEBYSCORE', KEYS[3], '-inf', now, 'LIMIT', 0, 128)
local envelope = redis.call('HGET', KEYS[1], token)
local score = redis.call('ZSCORE', KEYS[2], token)
if prior and tonumber(prior) > now then
  redis.call('ZREM', KEYS[2], token)
  redis.call('HDEL', KEYS[1], token)
  redis.call('PEXPIRE', KEYS[3], ttl)
  redis.call('ZREM', KEYS[4], token)
  return {1, 1, tostring(now), tostring(prior)}
end
if not envelope or not score or tonumber(score) <= now then return {0, 0, tostring(now)} end
if #expired > 0 then redis.call('ZREM', KEYS[3], unpack(expired)) end
local outcome = now + retention
redis.call('ZADD', KEYS[3], outcome, token)
redis.call('PEXPIRE', KEYS[3], ttl)
redis.call('ZREM', KEYS[2], token)
redis.call('HDEL', KEYS[1], token)
redis.call('ZREM', KEYS[4], token)
return {1, 0, tostring(now), tostring(outcome)}
`

var reliableAckScript = redis.NewScript(reliableAckScriptSource)

const reliableReleaseScriptSource = `
local function typename(key)
  local value = redis.call('TYPE', key)
  if type(value) == 'table' then return value['ok'] end
  return value
end
local function nowms()
  local now = redis.call('TIME')
  return tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
end
local rt = typename(KEYS[1])
local ht = typename(KEYS[2])
local vt = typename(KEYS[3])
local bt = typename(KEYS[4])
if (rt ~= 'none' and rt ~= 'list') or (ht ~= 'none' and ht ~= 'hash') or
   (vt ~= 'none' and vt ~= 'zset') or (bt ~= 'none' and bt ~= 'zset') then
  return redis.error_reply('reliable release: invalid key type')
end
local token = ARGV[1]
if not token or token == '' then return redis.error_reply('reliable release: invalid token') end
local now = nowms()
local envelope = redis.call('HGET', KEYS[2], token)
local score = redis.call('ZSCORE', KEYS[3], token)
if not envelope or not score or tonumber(score) <= now then return {0, tostring(now)} end
if string.len(envelope) < 11 or string.sub(envelope, 11, 11) ~= ':' or
   not tonumber(string.sub(envelope, 1, 10)) then
  return redis.error_reply('reliable release: invalid envelope')
end
local payload = string.sub(envelope, 12)
redis.call('RPUSH', KEYS[1], payload)
redis.call('ZREM', KEYS[3], token)
redis.call('HDEL', KEYS[2], token)
redis.call('ZREM', KEYS[4], token)
return {1, tostring(now)}
`

var reliableReleaseScript = redis.NewScript(reliableReleaseScriptSource)

const reliableDeferScriptSource = `
local function typename(key)
  local value = redis.call('TYPE', key)
  if type(value) == 'table' then return value['ok'] end
  return value
end
local function nowms()
  local now = redis.call('TIME')
  return tonumber(now[1]) * 1000 + math.floor(tonumber(now[2]) / 1000)
end
local ht = typename(KEYS[1])
local vt = typename(KEYS[2])
local ot = typename(KEYS[3])
local bt = typename(KEYS[4])
if (ht ~= 'none' and ht ~= 'hash') or (vt ~= 'none' and vt ~= 'zset') or
   (ot ~= 'none' and ot ~= 'zset') or (bt ~= 'none' and bt ~= 'zset') then
  return redis.error_reply('reliable retry: invalid key type')
end
local oldtoken = ARGV[1]
local newtoken = ARGV[2]
local delay = tonumber(ARGV[3])
if not oldtoken or oldtoken == '' or not newtoken or newtoken == '' or
   not delay or delay < 0 then return redis.error_reply('reliable retry: invalid argument') end
if redis.call('HEXISTS', KEYS[1], newtoken) == 1 or redis.call('ZSCORE', KEYS[2], newtoken) or
   redis.call('ZSCORE', KEYS[3], newtoken) or redis.call('ZSCORE', KEYS[4], newtoken) then return {2} end
local now = nowms()
local envelope = redis.call('HGET', KEYS[1], oldtoken)
local score = redis.call('ZSCORE', KEYS[2], oldtoken)
if not envelope or not score or tonumber(score) <= now then return {0, tostring(now)} end
if string.len(envelope) < 11 or string.sub(envelope, 11, 11) ~= ':' then
  return redis.error_reply('reliable retry: invalid envelope')
end
local failure = tonumber(string.sub(envelope, 1, 10))
if not failure then return redis.error_reply('reliable retry: invalid failure count') end
if failure < 9999999999 then failure = failure + 1 end
local payload = string.sub(envelope, 12)
local newenvelope = string.format('%010d:', failure) .. payload
local deadline = now + delay
redis.call('HSET', KEYS[1], newtoken, newenvelope)
redis.call('ZADD', KEYS[2], deadline, newtoken)
redis.call('ZREM', KEYS[2], oldtoken)
redis.call('HDEL', KEYS[1], oldtoken)
redis.call('ZREM', KEYS[4], oldtoken)
return {1, newtoken, newenvelope, tostring(now), tostring(deadline)}
`

var reliableDeferScript = redis.NewScript(reliableDeferScriptSource)

func (q *reliableQueue) claim(ctx context.Context) (*reliableDelivery, error) {
reconcile:
	for reconciled := 0; reconciled < maxClaimReconciliations; reconciled++ {
		for attempt := 0; attempt < maxDeliveryTokenAttempts; attempt++ {
			token, err := q.tokenSource.next()
			if err != nil {
				return nil, err
			}
			requestStarted := time.Now()
			result, err := reliableClaimScript.Run(ctx, q.client,
				[]string{q.keys.ready, q.keys.inflight, q.keys.visibility, q.keys.outcomes, q.keys.repairCursor, q.keys.repairBacklog},
				token, q.lease.Milliseconds(), ackOutcomeKeyTTL.Milliseconds()).Result()
			if err != nil {
				return nil, wrapRedisOperation("claim queued task", err)
			}
			values, err := reliableScriptValues(result)
			if err != nil {
				return nil, err
			}
			code, err := reliableScriptInt(values[0])
			if err != nil {
				return nil, err
			}
			switch code {
			case 0:
				return nil, nil
			case 1:
				if len(values) != 5 {
					return nil, fmt.Errorf("claim queued task: invalid script response")
				}
				envelope, err := reliableScriptBytes(values[2])
				if err != nil {
					return nil, err
				}
				failures, payload, err := decodeReliableEnvelope(envelope)
				if err != nil {
					return nil, err
				}
				serverMillis, err := reliableScriptInt(values[3])
				if err != nil {
					return nil, err
				}
				deadlineMillis, err := reliableScriptInt(values[4])
				if err != nil {
					return nil, err
				}
				delivery := &reliableDelivery{token: token, payload: payload, failures: failures}
				delivery.updateConfirmation(requestStarted, serverMillis, deadlineMillis)
				return delivery, nil
			case 2:
				continue
			case 3:
				continue reconcile
			default:
				return nil, fmt.Errorf("claim queued task: unknown script result %d", code)
			}
		}
		return nil, ErrDeliveryTokenCollision
	}
	return nil, fmt.Errorf("claim queued task: reconciliation limit reached")
}

func (q *reliableQueue) renew(ctx context.Context, delivery *reliableDelivery) error {
	requestStarted := time.Now()
	result, err := reliableRenewScript.Run(ctx, q.client,
		[]string{q.keys.inflight, q.keys.visibility}, delivery.token, q.lease.Milliseconds()).Result()
	if err != nil {
		return wrapRedisOperation("renew queued task", err)
	}
	values, err := reliableScriptValues(result)
	if err != nil {
		return err
	}
	code, err := reliableScriptInt(values[0])
	if err != nil {
		return err
	}
	if code != 1 {
		return ErrDeliveryLeaseLost
	}
	if len(values) != 3 {
		return fmt.Errorf("renew queued task: invalid script response")
	}
	serverMillis, err := reliableScriptInt(values[1])
	if err != nil {
		return err
	}
	deadlineMillis, err := reliableScriptInt(values[2])
	if err != nil {
		return err
	}
	delivery.updateConfirmation(requestStarted, serverMillis, deadlineMillis)
	return nil
}

func (q *reliableQueue) acknowledge(ctx context.Context, delivery *reliableDelivery) error {
	result, err := reliableAckScript.Run(ctx, q.client,
		[]string{q.keys.inflight, q.keys.visibility, q.keys.outcomes, q.keys.repairBacklog}, delivery.token,
		ackOutcomeRetention.Milliseconds(), ackOutcomeKeyTTL.Milliseconds()).Result()
	if err != nil {
		return wrapRedisOperation("acknowledge queued task", err)
	}
	values, err := reliableScriptValues(result)
	if err != nil {
		return err
	}
	code, err := reliableScriptInt(values[0])
	if err != nil {
		return err
	}
	if code != 1 {
		return ErrDeliveryLeaseLost
	}
	return nil
}

func (q *reliableQueue) release(ctx context.Context, delivery *reliableDelivery) error {
	result, err := reliableReleaseScript.Run(ctx, q.client,
		[]string{q.keys.ready, q.keys.inflight, q.keys.visibility, q.keys.repairBacklog}, delivery.token).Result()
	if err != nil {
		return wrapRedisOperation("release queued task", err)
	}
	values, err := reliableScriptValues(result)
	if err != nil {
		return err
	}
	code, err := reliableScriptInt(values[0])
	if err != nil {
		return err
	}
	if code != 1 {
		return ErrDeliveryLeaseLost
	}
	return nil
}

func (q *reliableQueue) deferRetry(ctx context.Context, delivery *reliableDelivery) (*reliableDelivery, error) {
	delay := reliableRetryDelay(delivery.failures, delivery.payload)
	for attempt := 0; attempt < maxDeliveryTokenAttempts; attempt++ {
		newToken, err := q.tokenSource.next()
		if err != nil {
			return nil, err
		}
		requestStarted := time.Now()
		result, err := reliableDeferScript.Run(ctx, q.client,
			[]string{q.keys.inflight, q.keys.visibility, q.keys.outcomes, q.keys.repairBacklog},
			delivery.token, newToken, delay.Milliseconds()).Result()
		if err != nil {
			return nil, wrapRedisOperation("defer queued task", err)
		}
		values, err := reliableScriptValues(result)
		if err != nil {
			return nil, err
		}
		code, err := reliableScriptInt(values[0])
		if err != nil {
			return nil, err
		}
		switch code {
		case 0:
			return nil, ErrDeliveryLeaseLost
		case 1:
			if len(values) != 5 {
				return nil, fmt.Errorf("defer queued task: invalid script response")
			}
			envelope, err := reliableScriptBytes(values[2])
			if err != nil {
				return nil, err
			}
			failures, payload, err := decodeReliableEnvelope(envelope)
			if err != nil {
				return nil, err
			}
			serverMillis, err := reliableScriptInt(values[3])
			if err != nil {
				return nil, err
			}
			deadlineMillis, err := reliableScriptInt(values[4])
			if err != nil {
				return nil, err
			}
			next := &reliableDelivery{token: newToken, payload: payload, failures: failures}
			next.updateConfirmation(requestStarted, serverMillis, deadlineMillis)
			return next, nil
		case 2:
			continue
		default:
			return nil, fmt.Errorf("defer queued task: unknown script result %d", code)
		}
	}
	return nil, ErrDeliveryTokenCollision
}

func reliableRetryDelay(failures uint64, payload []byte) time.Duration {
	exponent := failures
	if exponent > 6 {
		exponent = 6
	}
	base := time.Second * time.Duration(uint64(1)<<exponent)
	if base > 60*time.Second {
		base = 60 * time.Second
	}
	seedInput := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint64(seedInput[:8], failures)
	copy(seedInput[8:], payload)
	digest := sha256.Sum256(seedInput)
	// Deterministic factor in [0.8, 1.2].
	basisPoints := 8000 + int(binary.BigEndian.Uint16(digest[:2]))%4001
	delay := time.Duration(int64(base) * int64(basisPoints) / 10000)
	if delay > 60*time.Second {
		return 60 * time.Second
	}
	return delay
}

func decodeReliableEnvelope(envelope []byte) (uint64, []byte, error) {
	if len(envelope) < reliableEnvelopeHeaderSize || envelope[10] != ':' {
		return 0, nil, fmt.Errorf("reliable delivery: invalid envelope")
	}
	failures, err := strconv.ParseUint(string(envelope[:10]), 10, 64)
	if err != nil {
		return 0, nil, fmt.Errorf("reliable delivery: invalid failure count: %w", err)
	}
	payload := append([]byte(nil), envelope[reliableEnvelopeHeaderSize:]...)
	return failures, payload, nil
}

func reliableScriptValues(result interface{}) ([]interface{}, error) {
	values, ok := result.([]interface{})
	if !ok || len(values) == 0 {
		return nil, fmt.Errorf("reliable delivery: invalid script response %T", result)
	}
	return values, nil
}

func reliableScriptInt(value interface{}) (int64, error) {
	switch typed := value.(type) {
	case int64:
		return typed, nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("reliable delivery: invalid integer response: %w", err)
		}
		return parsed, nil
	case []byte:
		parsed, err := strconv.ParseInt(string(typed), 10, 64)
		if err != nil {
			return 0, fmt.Errorf("reliable delivery: invalid integer response: %w", err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("reliable delivery: invalid integer response %T", value)
	}
}

func reliableScriptBytes(value interface{}) ([]byte, error) {
	switch typed := value.(type) {
	case string:
		return []byte(typed), nil
	case []byte:
		return append([]byte(nil), typed...), nil
	default:
		return nil, fmt.Errorf("reliable delivery: invalid byte response %T", value)
	}
}
