package backend_redis

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
)

const (
	redisChordKeyPrefix        = "gkit:chord:"
	redisChordDeliveryIndexKey = redisChordKeyPrefix + "{index}:deliveries:v1"
	redisChordTerminalIndexKey = redisChordKeyPrefix + "{index}:terminal:v1"
	redisChordIndexStateKey    = redisChordKeyPrefix + "{index}:delivery-state:v1"
	redisChordCursorPrefix     = "chord-index-v1."
)

var errInvalidRedisChordRecord = errors.New("invalid redis chord record")

const (
	redisChordPrepareIndexScript = `
local current = redis.call('HGET', KEYS[2], ARGV[1])
if current and string.sub(current, 1, 9) == 'deleting:' then
  return current
end
redis.call('ZADD', KEYS[1], 0, ARGV[1])
if not current or string.sub(current, 1, 8) == 'pending:' then
  redis.call('HSET', KEYS[2], ARGV[1], 'pending:' .. ARGV[2])
end
return ''`
	redisChordCommitIndexScript = `
redis.call('ZADD', KEYS[1], 0, ARGV[1])
local current = redis.call('HGET', KEYS[2], ARGV[1])
if not current or current == 'pending:' .. ARGV[2] then
  redis.call('HSET', KEYS[2], ARGV[1], 'committed:' .. ARGV[3])
end
return 1`
	redisChordForceCommitIndexScript = `
redis.call('ZADD', KEYS[1], 0, ARGV[1])
redis.call('HSET', KEYS[2], ARGV[1], 'committed:' .. ARGV[2])
return 1`
	redisChordEnsureIndexScript = `
redis.call('ZADD', KEYS[1], 0, ARGV[1])
local current = redis.call('HGET', KEYS[2], ARGV[1])
if not current then
  redis.call('HSET', KEYS[2], ARGV[1], 'committed:' .. ARGV[2])
end
return 1`
	redisChordRemoveIndexScript = `
local current = redis.call('HGET', KEYS[3], ARGV[1])
if (ARGV[2] == '' and not current) or current == ARGV[2] then
  redis.call('ZREM', KEYS[1], ARGV[1])
  redis.call('ZREM', KEYS[2], ARGV[1])
  redis.call('HDEL', KEYS[3], ARGV[1])
  return 1
end
return 0`
	redisChordBeginDeleteScript = `
local current = redis.call('HGET', KEYS[1], ARGV[1])
if current and (string.sub(current, 1, 8) == 'pending:' or string.sub(current, 1, 9) == 'deleting:') then
  return 0
end
if current and current ~= 'committed:' .. ARGV[2] then
  return 0
end
redis.call('HSET', KEYS[1], ARGV[1], 'deleting:' .. ARGV[3])
return 1`
	redisChordRestoreIndexScript = `
if redis.call('HGET', KEYS[1], ARGV[1]) == 'deleting:' .. ARGV[2] then
  redis.call('HSET', KEYS[1], ARGV[1], 'committed:' .. ARGV[3])
  return 1
end
return 0`
	redisChordDeleteRecordScript = `
local body = redis.call('GET', KEYS[1])
if not body then
  return 0
end
local ok, record = pcall(cjson.decode, body)
if not ok or record['registration_owner'] ~= ARGV[1] then
  return 0
end
return redis.call('DEL', KEYS[1])`
)

var _ backend.DurableChordBackend = (*BackendRedis)(nil)

func redisChordRecordKey(deliveryKey string) (string, error) {
	parts := strings.Split(deliveryKey, ":")
	if len(parts) != 4 || parts[0] != "chord" || parts[1] != "v1" || parts[2] == "" {
		return "", fmt.Errorf("invalid chord delivery key %q", deliveryKey)
	}
	return redisChordKeyPrefix + "{" + parts[2] + "}:record", nil
}

func (b *BackendRedis) RegisterChord(ctx context.Context, registration backend.ChordRegistration) (backend.ChordRegistrationRef, error) {
	if err := backend.FinalizeChordRegistration(&registration); err != nil {
		return backend.ChordRegistrationRef{}, err
	}
	if err := validateRedisChordRegistration(registration); err != nil {
		return backend.ChordRegistrationRef{}, err
	}
	owner, err := backend.NewChordOwner()
	if err != nil {
		return backend.ChordRegistrationRef{}, err
	}
	key, err := redisChordRecordKey(registration.DeliveryKey)
	if err != nil {
		return backend.ChordRegistrationRef{}, err
	}
	delivery := backend.NewChordDelivery(registration, owner, time.Now())
	body, err := json.Marshal(&delivery)
	if err != nil {
		return backend.ChordRegistrationRef{}, fmt.Errorf("marshal chord registration: %w", err)
	}
	// Publish a pending index generation before the record. Scanners never
	// delete pending generations, and the final transition to committed fences
	// stale hole cleanup from removing a newly created record's index.
	for attempt := 0; ; attempt++ {
		state, prepareErr := b.prepareChordIndex(ctx, delivery.DeliveryKey, owner)
		if prepareErr != nil {
			return backend.ChordRegistrationRef{}, prepareErr
		}
		if !strings.HasPrefix(state, "deleting:") {
			break
		}
		if _, loadErr := b.loadChordDelivery(ctx, key); errors.Is(loadErr, redis.Nil) {
			if err := b.removeChordIndexesIfState(ctx, delivery.DeliveryKey, state); err != nil {
				return backend.ChordRegistrationRef{}, err
			}
			continue
		} else if loadErr != nil {
			return backend.ChordRegistrationRef{}, loadErr
		}
		if attempt == 15 {
			return backend.ChordRegistrationRef{}, errors.New("redis chord registration blocked by deletion")
		}
		timer := time.NewTimer(time.Duration(attempt+1) * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return backend.ChordRegistrationRef{}, ctx.Err()
		case <-timer.C:
		}
	}
	created, err := b.client.SetNX(ctx, key, body, 0).Result()
	if err != nil {
		return backend.ChordRegistrationRef{}, fmt.Errorf("register redis chord: %w", err)
	}
	if created {
		if err := b.forceCommitChordIndex(ctx, delivery.DeliveryKey, owner); err != nil {
			return backend.ChordRegistrationRef{}, err
		}
		return backend.ChordRegistrationRef{DeliveryKey: delivery.DeliveryKey, Owner: owner, Version: delivery.RegistrationVersion, Created: true}, nil
	}
	existing, err := b.loadChordDelivery(ctx, key)
	if err != nil {
		if errors.Is(err, errInvalidRedisChordRecord) {
			return backend.ChordRegistrationRef{}, b.rejectOccupiedChordRecord(ctx, delivery.DeliveryKey, owner, key)
		}
		return backend.ChordRegistrationRef{}, err
	}
	if !validRedisChordRecord(existing, key) {
		return backend.ChordRegistrationRef{}, b.rejectOccupiedChordRecord(ctx, delivery.DeliveryKey, owner, key)
	}
	if existing.DeliveryKey == delivery.DeliveryKey {
		if err := b.commitChordIndex(ctx, existing.DeliveryKey, owner, existing.RegistrationOwner); err != nil {
			return backend.ChordRegistrationRef{}, err
		}
	} else {
		if err := b.ensureChordIndex(ctx, existing.DeliveryKey, existing.RegistrationOwner); err != nil {
			return backend.ChordRegistrationRef{}, err
		}
		if err := b.removeChordIndexesIfState(ctx, delivery.DeliveryKey, "pending:"+owner); err != nil {
			return backend.ChordRegistrationRef{}, err
		}
	}
	if !backend.ChordRegistrationMatches(existing, registration) {
		return backend.ChordRegistrationRef{}, backend.ErrChordRegistrationConflict
	}
	return backend.ChordRegistrationRef{DeliveryKey: existing.DeliveryKey, Owner: existing.RegistrationOwner, Version: existing.RegistrationVersion, Created: false}, nil
}

func (b *BackendRedis) AbortRegistration(ctx context.Context, ref backend.ChordRegistrationRef) error {
	if !ref.Created {
		return backend.ErrChordRegistrationOwnershipLost
	}
	key, err := redisChordRecordKey(ref.DeliveryKey)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 8; attempt++ {
		deleteToken, tokenErr := backend.NewChordOwner()
		if tokenErr != nil {
			return tokenErr
		}
		deleteAcquired := false
		var deleteCmd *redis.Cmd
		err = b.client.Watch(ctx, func(tx *redis.Tx) error {
			delivery, loadErr := loadRedisChordFromCmd(ctx, tx, key)
			if errors.Is(loadErr, redis.Nil) {
				return nil
			}
			if loadErr != nil {
				return loadErr
			}
			if delivery.RegistrationOwner != ref.Owner || delivery.RegistrationVersion != ref.Version {
				return backend.ErrChordRegistrationOwnershipLost
			}
			if delivery.MemberPublicationStarted {
				return backend.ErrChordPublicationStarted
			}
			acquired, acquireErr := b.beginChordDelete(ctx, delivery.DeliveryKey, delivery.RegistrationOwner, deleteToken)
			if acquireErr != nil {
				return acquireErr
			}
			if !acquired {
				return redis.TxFailedErr
			}
			deleteAcquired = true
			_, pipelineErr := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				deleteCmd = pipe.Eval(ctx, redisChordDeleteRecordScript, []string{key}, delivery.RegistrationOwner)
				return nil
			})
			if pipelineErr != nil {
				_ = b.restoreChordIndex(ctx, delivery.DeliveryKey, deleteToken, delivery.RegistrationOwner)
			}
			return pipelineErr
		}, key)
		if !errors.Is(err, redis.TxFailedErr) {
			if err != nil {
				return err
			}
			if deleteAcquired && deleteCmd != nil {
				if _, deleteErr := deleteCmd.Int(); deleteErr != nil {
					_ = b.restoreChordIndex(ctx, ref.DeliveryKey, deleteToken, ref.Owner)
					return deleteErr
				}
			}
			if deleteAcquired {
				return b.removeChordIndexesIfState(ctx, ref.DeliveryKey, "deleting:"+deleteToken)
			}
			return b.removeChordIndexesIfState(ctx, ref.DeliveryKey, "committed:"+ref.Owner)
		}
		if deleteAcquired {
			_ = b.restoreChordIndex(ctx, ref.DeliveryKey, deleteToken, ref.Owner)
		}
	}
	return fmt.Errorf("abort redis chord: %w", redis.TxFailedErr)
}

func (b *BackendRedis) ClaimMemberPublication(ctx context.Context, claim backend.ChordMemberClaim) (lease backend.ChordMemberLease, claimed bool, err error) {
	key, err := redisChordRecordKey(claim.DeliveryKey)
	if err != nil {
		return lease, false, err
	}
	err = b.updateChordDelivery(ctx, key, func(delivery *backend.ChordDelivery) (bool, error) {
		var transitionErr error
		lease, claimed, transitionErr = backend.ClaimChordMember(delivery, claim)
		return claimed, transitionErr
	})
	return lease, claimed, err
}

func (b *BackendRedis) RecordMemberPublishOutcome(ctx context.Context, lease backend.ChordMemberLease, outcome backend.ChordPublishOutcome) error {
	key, err := redisChordRecordKey(lease.DeliveryKey)
	if err != nil {
		return err
	}
	return b.updateChordDelivery(ctx, key, func(delivery *backend.ChordDelivery) (bool, error) {
		if err := backend.ApplyChordMemberPublishOutcome(delivery, lease, outcome); err != nil {
			return false, err
		}
		return true, nil
	})
}

func (b *BackendRedis) RecordMemberTerminal(ctx context.Context, deliveryKey string, ordinal int, taskID string, outcome backend.MemberTerminalOutcome, results []*task.Result) error {
	key, err := redisChordRecordKey(deliveryKey)
	if err != nil {
		return err
	}
	return b.updateChordDelivery(ctx, key, func(delivery *backend.ChordDelivery) (bool, error) {
		before := delivery.Version
		if err := backend.ApplyChordMemberTerminal(delivery, ordinal, taskID, outcome, results, time.Now()); err != nil {
			return false, err
		}
		return delivery.Version != before, nil
	})
}

func (b *BackendRedis) ScanChordDeliveries(ctx context.Context, scan backend.ChordScan) (backend.ChordDeliveryPage, error) {
	limit := scan.Limit
	if limit <= 0 {
		limit = 100
	}
	lastKey, err := decodeRedisChordCursor(scan.Cursor)
	if err != nil {
		return backend.ChordDeliveryPage{}, err
	}
	minimum := "-"
	if lastKey != "" {
		minimum = "(" + lastKey
	}
	candidates, err := b.client.ZRangeByLex(ctx, redisChordDeliveryIndexKey, &redis.ZRangeBy{
		Min:    minimum,
		Max:    "+",
		Offset: 0,
		Count:  int64(limit + 1),
	}).Result()
	if err != nil {
		return backend.ChordDeliveryPage{}, fmt.Errorf("scan redis chord index: %w", err)
	}
	page := backend.ChordDeliveryPage{Deliveries: make([]backend.ChordDelivery, 0, limit)}
	processedLast := lastKey
	hasMore := false
	for _, deliveryKey := range candidates {
		if len(page.Deliveries) == limit {
			hasMore = true
			break
		}
		processedLast = deliveryKey
		key, keyErr := redisChordRecordKey(deliveryKey)
		if keyErr != nil {
			_ = b.removeChordIndexes(ctx, deliveryKey)
			continue
		}
		delivery, loadErr := b.loadChordDelivery(ctx, key)
		if errors.Is(loadErr, redis.Nil) {
			state, stateErr := b.chordIndexState(ctx, deliveryKey)
			if stateErr != nil {
				return backend.ChordDeliveryPage{}, stateErr
			}
			if strings.HasPrefix(state, "pending:") || strings.HasPrefix(state, "deleting:") {
				continue
			}
			// A registration may have created the record after the first GET.
			// Recheck before conditionally removing only the unchanged generation.
			delivery, loadErr = b.loadChordDelivery(ctx, key)
			if errors.Is(loadErr, redis.Nil) {
				if err := b.removeChordIndexesIfState(ctx, deliveryKey, state); err != nil {
					return backend.ChordDeliveryPage{}, err
				}
				continue
			}
			if loadErr != nil {
				return backend.ChordDeliveryPage{}, loadErr
			}
		}
		if loadErr != nil {
			return backend.ChordDeliveryPage{}, loadErr
		}
		if err := b.ensureChordIndex(ctx, delivery.DeliveryKey, delivery.RegistrationOwner); err != nil {
			return backend.ChordDeliveryPage{}, err
		}
		if delivery.DeliveryKey != deliveryKey {
			state, stateErr := b.chordIndexState(ctx, deliveryKey)
			if stateErr != nil {
				return backend.ChordDeliveryPage{}, stateErr
			}
			if err := b.removeChordIndexesIfState(ctx, deliveryKey, state); err != nil {
				return backend.ChordDeliveryPage{}, err
			}
			continue
		}
		page.Deliveries = append(page.Deliveries, *delivery)
	}
	if !hasMore && len(candidates) == limit+1 {
		hasMore = true
	}
	if hasMore && processedLast != "" {
		page.NextCursor = encodeRedisChordCursor(processedLast)
	}
	return page, nil
}

func (b *BackendRedis) ReconcileChord(ctx context.Context, deliveryKey string) error {
	key, err := redisChordRecordKey(deliveryKey)
	if err != nil {
		return err
	}
	delivery, err := b.loadChordDelivery(ctx, key)
	if err != nil {
		return err
	}
	if err := validateRedisChordDeliverySetup(delivery); err != nil {
		return err
	}
	needsSetup := false
	for index := range delivery.Members {
		if delivery.Members[index].State == backend.ChordMemberSetup {
			needsSetup = true
			break
		}
	}
	if !needsSetup {
		return nil
	}
	taskIDs := make([]string, len(delivery.Members))
	for index := range delivery.Members {
		taskIDs[index] = delivery.Members[index].TaskID
	}
	group, groupErr := b.getGroup(delivery.GroupID)
	if errors.Is(groupErr, redis.Nil) {
		if err := b.GroupTakeOver(delivery.GroupID, delivery.GroupName, taskIDs...); err != nil && !errors.Is(err, ErrGroupAlreadyExists) {
			return err
		}
	} else if groupErr != nil {
		return groupErr
	} else if !sameStrings([]string(group.TaskIDs), taskIDs) {
		return backend.ErrChordRegistrationConflict
	}
	for index := range delivery.Members {
		var signature task.Signature
		if err := json.Unmarshal(delivery.Members[index].Payload, &signature); err != nil {
			return fmt.Errorf("decode durable member %d: %w", index, err)
		}
		pending, err := json.Marshal(task.NewPendingState(&signature))
		if err != nil {
			return err
		}
		expiration := b.resultExpire
		if expiration < 0 {
			expiration = 0
		}
		created, err := b.client.SetNX(ctx, signature.ID, pending, time.Duration(expiration)*time.Second).Result()
		if err != nil {
			return err
		}
		if !created {
			value, err := b.client.Get(ctx, signature.ID).Bytes()
			if err != nil {
				return err
			}
			var existing task.Status
			if err := json.Unmarshal(value, &existing); err != nil {
				return err
			}
			if existing.GroupID != delivery.GroupID {
				return backend.ErrChordRegistrationConflict
			}
		}
	}
	return b.updateChordDelivery(ctx, key, func(current *backend.ChordDelivery) (bool, error) {
		backend.PrepareChordMembers(current, time.Now())
		return true, nil
	})
}

func (b *BackendRedis) ClaimCallbackPublication(ctx context.Context, claim backend.ChordCallbackClaim) (lease backend.ChordCallbackLease, claimed bool, err error) {
	key, err := redisChordRecordKey(claim.DeliveryKey)
	if err != nil {
		return lease, false, err
	}
	delivery, err := b.loadChordDelivery(ctx, key)
	if err != nil {
		return lease, false, err
	}
	if len(delivery.CallbackPayload) > 0 {
		var callback task.Signature
		if err := json.Unmarshal(delivery.CallbackPayload, &callback); err != nil {
			return lease, false, err
		}
		if err := validateRedisUserKey("callback", callback.ID); err != nil {
			return lease, false, err
		}
		pending, err := json.Marshal(task.NewPendingState(&callback))
		if err != nil {
			return lease, false, err
		}
		expiration := b.resultExpire
		if expiration < 0 {
			expiration = 0
		}
		if _, err := b.client.SetNX(ctx, callback.ID, pending, time.Duration(expiration)*time.Second).Result(); err != nil {
			return lease, false, err
		}
	}
	err = b.updateChordDelivery(ctx, key, func(current *backend.ChordDelivery) (bool, error) {
		var transitionErr error
		lease, claimed, transitionErr = backend.ClaimChordCallback(current, claim)
		return claimed, transitionErr
	})
	return lease, claimed, err
}

func (b *BackendRedis) RecordCallbackPublishOutcome(ctx context.Context, lease backend.ChordCallbackLease, outcome backend.ChordPublishOutcome) error {
	key, err := redisChordRecordKey(lease.DeliveryKey)
	if err != nil {
		return err
	}
	return b.updateChordDelivery(ctx, key, func(delivery *backend.ChordDelivery) (bool, error) {
		if err := backend.ApplyChordCallbackPublishOutcome(delivery, lease, outcome); err != nil {
			return false, err
		}
		return true, nil
	})
}

func (b *BackendRedis) RecordCallbackTerminal(ctx context.Context, deliveryKey string, outcome backend.CallbackTerminalOutcome) error {
	key, err := redisChordRecordKey(deliveryKey)
	if err != nil {
		return err
	}
	return b.updateChordDelivery(ctx, key, func(delivery *backend.ChordDelivery) (bool, error) {
		before := delivery.Version
		if err := backend.ApplyChordCallbackTerminal(delivery, outcome, time.Now()); err != nil {
			return false, err
		}
		return delivery.Version != before, nil
	})
}

func (b *BackendRedis) CleanupTerminalChordDeliveries(ctx context.Context, now time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	due, err := b.client.ZRangeByScore(ctx, redisChordTerminalIndexKey, &redis.ZRangeBy{
		Min:    "-inf",
		Max:    fmt.Sprintf("%d", now.UnixMilli()),
		Offset: 0,
		Count:  int64(limit),
	}).Result()
	if err != nil {
		return 0, fmt.Errorf("scan redis terminal chord index: %w", err)
	}
	removed := 0
	for _, deliveryKey := range due {
		key, keyErr := redisChordRecordKey(deliveryKey)
		if keyErr != nil {
			_ = b.removeChordIndexes(ctx, deliveryKey)
			continue
		}
		delivery, loadErr := b.loadChordDelivery(ctx, key)
		if errors.Is(loadErr, redis.Nil) {
			state, stateErr := b.chordIndexState(ctx, deliveryKey)
			if stateErr != nil {
				return removed, stateErr
			}
			if strings.HasPrefix(state, "pending:") {
				continue
			}
			if err := b.removeChordIndexesIfState(ctx, deliveryKey, state); err != nil {
				return removed, err
			}
			continue
		}
		if loadErr != nil {
			return removed, loadErr
		}
		if delivery.TerminalExpireAt == nil {
			if err := b.client.ZRem(ctx, redisChordTerminalIndexKey, deliveryKey).Err(); err != nil {
				return removed, err
			}
			continue
		}
		if delivery.TerminalExpireAt.After(now) {
			if err := b.client.ZAdd(ctx, redisChordTerminalIndexKey, &redis.Z{Score: float64(delivery.TerminalExpireAt.UnixMilli()), Member: deliveryKey}).Err(); err != nil {
				return removed, err
			}
			continue
		}
		deleteToken, tokenErr := backend.NewChordOwner()
		if tokenErr != nil {
			return removed, tokenErr
		}
		acquired, acquireErr := b.beginChordDelete(ctx, deliveryKey, delivery.RegistrationOwner, deleteToken)
		if acquireErr != nil {
			return removed, acquireErr
		}
		if !acquired {
			continue
		}
		deleted, err := b.deleteChordRecordIfOwner(ctx, key, delivery.RegistrationOwner)
		if err != nil {
			_ = b.restoreChordIndex(ctx, deliveryKey, deleteToken, delivery.RegistrationOwner)
			return removed, err
		}
		if err := b.removeChordIndexesIfState(ctx, deliveryKey, "deleting:"+deleteToken); err != nil {
			return removed, err
		}
		if deleted > 0 {
			removed++
		}
	}
	return removed, nil
}

func (b *BackendRedis) updateChordDelivery(ctx context.Context, key string, mutate func(*backend.ChordDelivery) (bool, error)) error {
	for attempt := 0; attempt < 16; attempt++ {
		var terminalDeliveryKey string
		var terminalExpireAt *time.Time
		err := b.client.Watch(ctx, func(tx *redis.Tx) error {
			delivery, err := loadRedisChordFromCmd(ctx, tx, key)
			if err != nil {
				if errors.Is(err, redis.Nil) {
					return backend.ErrChordNotFound
				}
				return err
			}
			changed, err := mutate(delivery)
			if err != nil {
				return err
			}
			terminalDeliveryKey = delivery.DeliveryKey
			terminalExpireAt = delivery.TerminalExpireAt
			if !changed {
				return nil
			}
			body, err := json.Marshal(delivery)
			if err != nil {
				return err
			}
			expiration := time.Duration(0)
			if delivery.TerminalExpireAt != nil {
				expiration = time.Until(*delivery.TerminalExpireAt)
				if expiration <= 0 {
					expiration = time.Millisecond
				}
			}
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, key, body, expiration)
				return nil
			})
			return err
		}, key)
		if !errors.Is(err, redis.TxFailedErr) {
			if err != nil {
				return err
			}
			if terminalExpireAt != nil {
				if err := b.client.ZAdd(ctx, redisChordTerminalIndexKey, &redis.Z{Score: float64(terminalExpireAt.UnixMilli()), Member: terminalDeliveryKey}).Err(); err != nil {
					return fmt.Errorf("index terminal redis chord: %w", err)
				}
			}
			return nil
		}
	}
	return fmt.Errorf("redis chord CAS exhausted: %w", redis.TxFailedErr)
}

func (b *BackendRedis) loadChordDelivery(ctx context.Context, key string) (*backend.ChordDelivery, error) {
	return loadRedisChordFromCmd(ctx, b.client, key)
}

type redisChordGetter interface {
	Get(context.Context, string) *redis.StringCmd
}

func loadRedisChordFromCmd(ctx context.Context, getter redisChordGetter, key string) (*backend.ChordDelivery, error) {
	body, err := getter.Get(ctx, key).Bytes()
	if err != nil {
		return nil, err
	}
	var delivery backend.ChordDelivery
	if err := json.Unmarshal(body, &delivery); err != nil {
		return nil, fmt.Errorf("%w: decode redis chord %q: %v", errInvalidRedisChordRecord, key, err)
	}
	return &delivery, nil
}

func validateRedisChordRegistration(registration backend.ChordRegistration) error {
	if err := validateRedisUserKey("group", registration.GroupID); err != nil {
		return err
	}
	for index, member := range registration.Members {
		if err := validateRedisUserKey(fmt.Sprintf("member %d", index), member.TaskID); err != nil {
			return err
		}
		var signature task.Signature
		if err := json.Unmarshal(member.Payload, &signature); err != nil {
			return fmt.Errorf("%w: decode redis member %d: %v", backend.ErrChordInvalidInput, index, err)
		}
		if signature.ID != member.TaskID {
			return fmt.Errorf("%w: redis member %d payload id %q does not match registration id %q", backend.ErrChordInvalidInput, index, signature.ID, member.TaskID)
		}
		if err := validateRedisUserKey(fmt.Sprintf("member %d", index), signature.ID); err != nil {
			return err
		}
	}
	var callback task.Signature
	if err := json.Unmarshal(registration.Callback, &callback); err != nil {
		return fmt.Errorf("%w: decode redis callback: %v", backend.ErrChordInvalidInput, err)
	}
	return validateRedisUserKey("callback", callback.ID)
}

func validateRedisChordDeliverySetup(delivery *backend.ChordDelivery) error {
	if delivery == nil {
		return fmt.Errorf("%w: nil redis chord delivery", backend.ErrChordInvalidInput)
	}
	if err := validateRedisUserKey("group", delivery.GroupID); err != nil {
		return err
	}
	for index, member := range delivery.Members {
		if err := validateRedisUserKey(fmt.Sprintf("member %d", index), member.TaskID); err != nil {
			return err
		}
		var signature task.Signature
		if err := json.Unmarshal(member.Payload, &signature); err != nil {
			return fmt.Errorf("%w: decode redis member %d: %v", backend.ErrChordInvalidInput, index, err)
		}
		if signature.ID != member.TaskID {
			return fmt.Errorf("%w: redis member %d payload id %q does not match stored id %q", backend.ErrChordInvalidInput, index, signature.ID, member.TaskID)
		}
		if err := validateRedisUserKey(fmt.Sprintf("member %d", index), signature.ID); err != nil {
			return err
		}
	}
	return nil
}

func validRedisChordRecord(delivery *backend.ChordDelivery, recordKey string) bool {
	if delivery == nil || delivery.DeliveryKey == "" || delivery.GroupID == "" || delivery.RegistrationOwner == "" || delivery.RegistrationVersion <= 0 {
		return false
	}
	key, err := redisChordRecordKey(delivery.DeliveryKey)
	return err == nil && key == recordKey
}

func (b *BackendRedis) rejectOccupiedChordRecord(ctx context.Context, deliveryKey, owner, recordKey string) error {
	collisionErr := fmt.Errorf("%w: redis chord record %q is occupied by incompatible data", backend.ErrChordRegistrationConflict, recordKey)
	cleanupErr := b.removeChordIndexesIfState(ctx, deliveryKey, "pending:"+owner)
	return errors.Join(collisionErr, cleanupErr)
}

func encodeRedisChordCursor(lastKey string) string {
	return redisChordCursorPrefix + base64.RawURLEncoding.EncodeToString([]byte(lastKey))
}

func decodeRedisChordCursor(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	if !strings.HasPrefix(cursor, redisChordCursorPrefix) {
		return "", errors.New("invalid redis chord cursor")
	}
	value, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(cursor, redisChordCursorPrefix))
	if err != nil {
		return "", errors.New("invalid redis chord cursor")
	}
	lastKey := string(value)
	if _, err := redisChordRecordKey(lastKey); err != nil {
		return "", errors.New("invalid redis chord cursor")
	}
	return lastKey, nil
}

func (b *BackendRedis) prepareChordIndex(ctx context.Context, deliveryKey, owner string) (string, error) {
	value, err := b.client.Eval(ctx, redisChordPrepareIndexScript,
		[]string{redisChordDeliveryIndexKey, redisChordIndexStateKey}, deliveryKey, owner).Text()
	if err != nil {
		return "", fmt.Errorf("prepare redis chord index: %w", err)
	}
	return value, nil
}

func (b *BackendRedis) commitChordIndex(ctx context.Context, deliveryKey, expectedOwner, recordOwner string) error {
	if err := b.client.Eval(ctx, redisChordCommitIndexScript,
		[]string{redisChordDeliveryIndexKey, redisChordIndexStateKey}, deliveryKey, expectedOwner, recordOwner).Err(); err != nil {
		return fmt.Errorf("commit redis chord index: %w", err)
	}
	return nil
}

func (b *BackendRedis) forceCommitChordIndex(ctx context.Context, deliveryKey, recordOwner string) error {
	if err := b.client.Eval(ctx, redisChordForceCommitIndexScript,
		[]string{redisChordDeliveryIndexKey, redisChordIndexStateKey}, deliveryKey, recordOwner).Err(); err != nil {
		return fmt.Errorf("commit created redis chord index: %w", err)
	}
	return nil
}

func (b *BackendRedis) ensureChordIndex(ctx context.Context, deliveryKey, recordOwner string) error {
	if err := b.client.Eval(ctx, redisChordEnsureIndexScript,
		[]string{redisChordDeliveryIndexKey, redisChordIndexStateKey}, deliveryKey, recordOwner).Err(); err != nil {
		return fmt.Errorf("repair redis chord index: %w", err)
	}
	return nil
}

func (b *BackendRedis) chordIndexState(ctx context.Context, deliveryKey string) (string, error) {
	state, err := b.client.HGet(ctx, redisChordIndexStateKey, deliveryKey).Result()
	if errors.Is(err, redis.Nil) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("read redis chord index state: %w", err)
	}
	return state, nil
}

func (b *BackendRedis) beginChordDelete(ctx context.Context, deliveryKey, recordOwner, deleteToken string) (bool, error) {
	acquired, err := b.client.Eval(ctx, redisChordBeginDeleteScript,
		[]string{redisChordIndexStateKey}, deliveryKey, recordOwner, deleteToken).Int()
	if err != nil {
		return false, fmt.Errorf("fence redis chord deletion: %w", err)
	}
	return acquired == 1, nil
}

func (b *BackendRedis) restoreChordIndex(ctx context.Context, deliveryKey, deleteToken, recordOwner string) error {
	if err := b.client.Eval(ctx, redisChordRestoreIndexScript,
		[]string{redisChordIndexStateKey}, deliveryKey, deleteToken, recordOwner).Err(); err != nil {
		return fmt.Errorf("restore redis chord index: %w", err)
	}
	return nil
}

func (b *BackendRedis) deleteChordRecordIfOwner(ctx context.Context, recordKey, recordOwner string) (int64, error) {
	deleted, err := b.client.Eval(ctx, redisChordDeleteRecordScript, []string{recordKey}, recordOwner).Int64()
	if err != nil {
		return 0, fmt.Errorf("delete redis chord record: %w", err)
	}
	return deleted, nil
}

func (b *BackendRedis) removeChordIndexesIfState(ctx context.Context, deliveryKey, expectedState string) error {
	if err := b.client.Eval(ctx, redisChordRemoveIndexScript,
		[]string{redisChordDeliveryIndexKey, redisChordTerminalIndexKey, redisChordIndexStateKey}, deliveryKey, expectedState).Err(); err != nil {
		return fmt.Errorf("remove redis chord indexes: %w", err)
	}
	return nil
}

func (b *BackendRedis) removeChordIndexes(ctx context.Context, deliveryKey string) error {
	if err := b.client.Eval(ctx, `
redis.call('ZREM', KEYS[1], ARGV[1])
redis.call('ZREM', KEYS[2], ARGV[1])
redis.call('HDEL', KEYS[3], ARGV[1])
return 1`, []string{redisChordDeliveryIndexKey, redisChordTerminalIndexKey, redisChordIndexStateKey}, deliveryKey).Err(); err != nil {
		return fmt.Errorf("remove redis chord indexes: %w", err)
	}
	return nil
}

func sameStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
