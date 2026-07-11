package backend_redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
)

const redisChordRecordPattern = "gkit:chord:*:record"

var _ backend.DurableChordBackend = (*BackendRedis)(nil)

func redisChordRecordKey(deliveryKey string) (string, error) {
	parts := strings.Split(deliveryKey, ":")
	if len(parts) != 4 || parts[0] != "chord" || parts[1] != "v1" || parts[2] == "" {
		return "", fmt.Errorf("invalid chord delivery key %q", deliveryKey)
	}
	return "gkit:chord:{" + parts[2] + "}:record", nil
}

func (b *BackendRedis) RegisterChord(ctx context.Context, registration backend.ChordRegistration) (backend.ChordRegistrationRef, error) {
	if err := backend.FinalizeChordRegistration(&registration); err != nil {
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
	created, err := b.client.SetNX(ctx, key, body, 0).Result()
	if err != nil {
		return backend.ChordRegistrationRef{}, fmt.Errorf("register redis chord: %w", err)
	}
	if created {
		return backend.ChordRegistrationRef{DeliveryKey: delivery.DeliveryKey, Owner: owner, Version: delivery.RegistrationVersion, Created: true}, nil
	}
	existing, err := b.loadChordDelivery(ctx, key)
	if err != nil {
		return backend.ChordRegistrationRef{}, err
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
			_, pipelineErr := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Del(ctx, key)
				return nil
			})
			return pipelineErr
		}, key)
		if !errors.Is(err, redis.TxFailedErr) {
			return err
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
	keys, err := b.scanChordRecordKeys(ctx)
	if err != nil {
		return backend.ChordDeliveryPage{}, err
	}
	deliveries := make([]backend.ChordDelivery, 0, len(keys))
	for _, key := range keys {
		delivery, loadErr := b.loadChordDelivery(ctx, key)
		if errors.Is(loadErr, redis.Nil) {
			continue
		}
		if loadErr != nil {
			return backend.ChordDeliveryPage{}, loadErr
		}
		if scan.Cursor == "" || delivery.DeliveryKey > scan.Cursor {
			deliveries = append(deliveries, *delivery)
		}
	}
	backend.SortChordDeliveries(deliveries)
	limit := scan.Limit
	if limit <= 0 {
		limit = 100
	}
	page := backend.ChordDeliveryPage{}
	if len(deliveries) > limit {
		page.Deliveries = deliveries[:limit]
		page.NextCursor = page.Deliveries[len(page.Deliveries)-1].DeliveryKey
	} else {
		page.Deliveries = deliveries
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
	keys, err := b.scanChordRecordKeys(ctx)
	if err != nil {
		return 0, err
	}
	if limit <= 0 {
		limit = 100
	}
	removed := 0
	for _, key := range keys {
		if removed >= limit {
			break
		}
		delivery, loadErr := b.loadChordDelivery(ctx, key)
		if errors.Is(loadErr, redis.Nil) {
			continue
		}
		if loadErr != nil {
			return removed, loadErr
		}
		if delivery.TerminalExpireAt != nil && !delivery.TerminalExpireAt.After(now) {
			if err := b.client.Del(ctx, key).Err(); err != nil {
				return removed, err
			}
			removed++
		}
	}
	return removed, nil
}

func (b *BackendRedis) updateChordDelivery(ctx context.Context, key string, mutate func(*backend.ChordDelivery) (bool, error)) error {
	for attempt := 0; attempt < 16; attempt++ {
		err := b.client.Watch(ctx, func(tx *redis.Tx) error {
			delivery, err := loadRedisChordFromCmd(ctx, tx, key)
			if err != nil {
				if errors.Is(err, redis.Nil) {
					return backend.ErrChordNotFound
				}
				return err
			}
			changed, err := mutate(delivery)
			if err != nil || !changed {
				return err
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
			return err
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
		return nil, fmt.Errorf("decode redis chord %q: %w", key, err)
	}
	return &delivery, nil
}

func (b *BackendRedis) scanChordRecordKeys(ctx context.Context) ([]string, error) {
	keys := make(map[string]struct{})
	scanClient := func(client redis.Cmdable) error {
		var cursor uint64
		for {
			batch, next, err := client.Scan(ctx, cursor, redisChordRecordPattern, 100).Result()
			if err != nil {
				return err
			}
			for _, key := range batch {
				keys[key] = struct{}{}
			}
			cursor = next
			if cursor == 0 {
				return nil
			}
		}
	}
	if cluster, ok := b.client.(*redis.ClusterClient); ok {
		if err := cluster.ForEachMaster(ctx, func(ctx context.Context, client *redis.Client) error {
			return scanClient(client)
		}); err != nil {
			return nil, err
		}
	} else if err := scanClient(b.client); err != nil {
		return nil, err
	}
	result := make([]string, 0, len(keys))
	for key := range keys {
		result = append(result, key)
	}
	sort.Strings(result)
	return result, nil
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
