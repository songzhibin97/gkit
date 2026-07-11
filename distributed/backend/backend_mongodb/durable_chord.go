package backend_mongodb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var _ backend.DurableChordBackend = (*BackendMongoDB)(nil)

type mongoChordDocument struct {
	ID                    string `bson:"_id"`
	backend.ChordDelivery `bson:",inline"`
}

func (b *BackendMongoDB) RegisterChord(ctx context.Context, registration backend.ChordRegistration) (backend.ChordRegistrationRef, error) {
	if err := b.ensureChordIndexes(ctx); err != nil {
		return backend.ChordRegistrationRef{}, err
	}
	if err := backend.FinalizeChordRegistration(&registration); err != nil {
		return backend.ChordRegistrationRef{}, err
	}
	owner, err := backend.NewChordOwner()
	if err != nil {
		return backend.ChordRegistrationRef{}, err
	}
	delivery := backend.NewChordDelivery(registration, owner, time.Now())
	document := mongoChordDocument{ID: delivery.DeliveryKey, ChordDelivery: delivery}
	if _, err := b.chordTable.InsertOne(ctx, &document); err == nil {
		return backend.ChordRegistrationRef{DeliveryKey: delivery.DeliveryKey, Owner: owner, Version: delivery.RegistrationVersion, Created: true}, nil
	} else if !mongo.IsDuplicateKeyError(err) {
		return backend.ChordRegistrationRef{}, fmt.Errorf("insert chord registration: %w", err)
	}
	var existing mongoChordDocument
	if err := b.chordTable.FindOne(ctx, bson.M{"group_id": registration.GroupID}).Decode(&existing); err != nil {
		return backend.ChordRegistrationRef{}, err
	}
	if !backend.ChordRegistrationMatches(&existing.ChordDelivery, registration) {
		return backend.ChordRegistrationRef{}, backend.ErrChordRegistrationConflict
	}
	return backend.ChordRegistrationRef{DeliveryKey: existing.DeliveryKey, Owner: existing.RegistrationOwner, Version: existing.RegistrationVersion, Created: false}, nil
}

func (b *BackendMongoDB) AbortRegistration(ctx context.Context, ref backend.ChordRegistrationRef) error {
	if !ref.Created {
		return backend.ErrChordRegistrationOwnershipLost
	}
	result, err := b.chordTable.DeleteOne(ctx, bson.M{
		"_id":                        ref.DeliveryKey,
		"registration_owner":         ref.Owner,
		"registration_version":       ref.Version,
		"member_publication_started": false,
	})
	if err != nil {
		return err
	}
	if result.DeletedCount == 1 {
		return nil
	}
	var existing mongoChordDocument
	if err := b.chordTable.FindOne(ctx, bson.M{"_id": ref.DeliveryKey}).Decode(&existing); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil
		}
		return err
	}
	if existing.RegistrationOwner != ref.Owner || existing.RegistrationVersion != ref.Version {
		return backend.ErrChordRegistrationOwnershipLost
	}
	return backend.ErrChordPublicationStarted
}

func (b *BackendMongoDB) ClaimMemberPublication(ctx context.Context, claim backend.ChordMemberClaim) (lease backend.ChordMemberLease, claimed bool, err error) {
	err = b.updateChordDelivery(ctx, claim.DeliveryKey, func(delivery *backend.ChordDelivery) (bool, error) {
		var transitionErr error
		lease, claimed, transitionErr = backend.ClaimChordMember(delivery, claim)
		return claimed, transitionErr
	})
	return lease, claimed, err
}

func (b *BackendMongoDB) RecordMemberPublishOutcome(ctx context.Context, lease backend.ChordMemberLease, outcome backend.ChordPublishOutcome) error {
	return b.updateChordDelivery(ctx, lease.DeliveryKey, func(delivery *backend.ChordDelivery) (bool, error) {
		if err := backend.ApplyChordMemberPublishOutcome(delivery, lease, outcome); err != nil {
			return false, err
		}
		return true, nil
	})
}

func (b *BackendMongoDB) RecordMemberTerminal(ctx context.Context, deliveryKey string, ordinal int, taskID string, outcome backend.MemberTerminalOutcome, results []*task.Result) error {
	return b.updateChordDelivery(ctx, deliveryKey, func(delivery *backend.ChordDelivery) (bool, error) {
		before := delivery.Version
		if err := backend.ApplyChordMemberTerminal(delivery, ordinal, taskID, outcome, results, time.Now()); err != nil {
			return false, err
		}
		return delivery.Version != before, nil
	})
}

func (b *BackendMongoDB) ScanChordDeliveries(ctx context.Context, scan backend.ChordScan) (backend.ChordDeliveryPage, error) {
	if err := b.ensureChordIndexes(ctx); err != nil {
		return backend.ChordDeliveryPage{}, err
	}
	limit := scan.Limit
	if limit <= 0 {
		limit = 100
	}
	filter := bson.M{}
	if scan.Cursor != "" {
		filter["_id"] = bson.M{"$gt": scan.Cursor}
	}
	cursor, err := b.chordTable.Find(ctx, filter, options.Find().SetSort(bson.D{{Key: "_id", Value: 1}}).SetLimit(int64(limit+1)))
	if err != nil {
		return backend.ChordDeliveryPage{}, err
	}
	defer cursor.Close(ctx)
	page := backend.ChordDeliveryPage{}
	for cursor.Next(ctx) {
		var document mongoChordDocument
		if err := cursor.Decode(&document); err != nil {
			return backend.ChordDeliveryPage{}, err
		}
		page.Deliveries = append(page.Deliveries, document.ChordDelivery)
	}
	if err := cursor.Err(); err != nil {
		return backend.ChordDeliveryPage{}, err
	}
	if len(page.Deliveries) > limit {
		page.Deliveries = page.Deliveries[:limit]
		page.NextCursor = page.Deliveries[len(page.Deliveries)-1].DeliveryKey
	}
	return page, nil
}

func (b *BackendMongoDB) ReconcileChord(ctx context.Context, deliveryKey string) error {
	delivery, err := b.getChordDelivery(ctx, deliveryKey)
	if err != nil {
		return err
	}
	needsSetup := false
	for index := range delivery.Members {
		if delivery.Members[index].State == backend.ChordMemberSetup {
			needsSetup = true
		}
	}
	if !needsSetup {
		return nil
	}
	ids := make([]string, len(delivery.Members))
	for index := range delivery.Members {
		ids[index] = delivery.Members[index].TaskID
	}
	group := task.InitGroupMeta(delivery.GroupID, delivery.GroupName, b.resultExpire, ids...)
	if _, err := b.groupTable.UpdateOne(ctx, bson.M{"_id": delivery.GroupID}, bson.M{"$setOnInsert": group}, options.Update().SetUpsert(true)); err != nil {
		return err
	}
	var existingGroup task.GroupMeta
	if err := b.groupTable.FindOne(ctx, bson.M{"_id": delivery.GroupID}).Decode(&existingGroup); err != nil {
		return err
	}
	if !sameChordStrings([]string(existingGroup.TaskIDs), ids) {
		return backend.ErrChordRegistrationConflict
	}
	for index := range delivery.Members {
		var signature task.Signature
		if err := json.Unmarshal(delivery.Members[index].Payload, &signature); err != nil {
			return err
		}
		status := task.NewPendingState(&signature)
		if _, err := b.taskTable.UpdateOne(ctx, bson.M{"_id": signature.ID}, bson.M{"$setOnInsert": status}, options.Update().SetUpsert(true)); err != nil {
			return err
		}
		var existingStatus task.Status
		if err := b.taskTable.FindOne(ctx, bson.M{"_id": signature.ID}).Decode(&existingStatus); err != nil {
			return err
		}
		if existingStatus.GroupID != delivery.GroupID {
			return backend.ErrChordRegistrationConflict
		}
	}
	return b.updateChordDelivery(ctx, deliveryKey, func(current *backend.ChordDelivery) (bool, error) {
		backend.PrepareChordMembers(current, time.Now())
		return true, nil
	})
}

func sameChordStrings(left, right []string) bool {
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

func (b *BackendMongoDB) ClaimCallbackPublication(ctx context.Context, claim backend.ChordCallbackClaim) (lease backend.ChordCallbackLease, claimed bool, err error) {
	delivery, err := b.getChordDelivery(ctx, claim.DeliveryKey)
	if err != nil {
		return lease, false, err
	}
	if len(delivery.CallbackPayload) > 0 {
		var callback task.Signature
		if err := json.Unmarshal(delivery.CallbackPayload, &callback); err != nil {
			return lease, false, err
		}
		status := task.NewPendingState(&callback)
		if _, err := b.taskTable.UpdateOne(ctx, bson.M{"_id": callback.ID}, bson.M{"$setOnInsert": status}, options.Update().SetUpsert(true)); err != nil {
			return lease, false, err
		}
	}
	err = b.updateChordDelivery(ctx, claim.DeliveryKey, func(current *backend.ChordDelivery) (bool, error) {
		var transitionErr error
		lease, claimed, transitionErr = backend.ClaimChordCallback(current, claim)
		return claimed, transitionErr
	})
	return lease, claimed, err
}

func (b *BackendMongoDB) RecordCallbackPublishOutcome(ctx context.Context, lease backend.ChordCallbackLease, outcome backend.ChordPublishOutcome) error {
	return b.updateChordDelivery(ctx, lease.DeliveryKey, func(delivery *backend.ChordDelivery) (bool, error) {
		if err := backend.ApplyChordCallbackPublishOutcome(delivery, lease, outcome); err != nil {
			return false, err
		}
		return true, nil
	})
}

func (b *BackendMongoDB) RecordCallbackTerminal(ctx context.Context, deliveryKey string, outcome backend.CallbackTerminalOutcome) error {
	return b.updateChordDelivery(ctx, deliveryKey, func(delivery *backend.ChordDelivery) (bool, error) {
		before := delivery.Version
		if err := backend.ApplyChordCallbackTerminal(delivery, outcome, time.Now()); err != nil {
			return false, err
		}
		return delivery.Version != before, nil
	})
}

func (b *BackendMongoDB) CleanupTerminalChordDeliveries(ctx context.Context, now time.Time, limit int) (int, error) {
	if err := b.ensureChordIndexes(ctx); err != nil {
		return 0, err
	}
	if limit <= 0 {
		limit = 100
	}
	cursor, err := b.chordTable.Find(ctx, bson.M{"terminal_expire_at": bson.M{"$lte": now}}, options.Find().SetProjection(bson.M{"_id": 1}).SetLimit(int64(limit)))
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)
	ids := make([]string, 0, limit)
	for cursor.Next(ctx) {
		var value struct {
			ID string `bson:"_id"`
		}
		if err := cursor.Decode(&value); err != nil {
			return len(ids), err
		}
		ids = append(ids, value.ID)
	}
	if len(ids) == 0 {
		return 0, cursor.Err()
	}
	result, err := b.chordTable.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": ids}})
	if err != nil {
		return 0, fmt.Errorf("delete terminal chord deliveries: %w", err)
	}
	return int(result.DeletedCount), nil
}

func (b *BackendMongoDB) getChordDelivery(ctx context.Context, deliveryKey string) (*backend.ChordDelivery, error) {
	if err := b.ensureChordIndexes(ctx); err != nil {
		return nil, err
	}
	var document mongoChordDocument
	if err := b.chordTable.FindOne(ctx, bson.M{"_id": deliveryKey}).Decode(&document); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, backend.ErrChordNotFound
		}
		return nil, err
	}
	return &document.ChordDelivery, nil
}

func (b *BackendMongoDB) updateChordDelivery(ctx context.Context, deliveryKey string, mutate func(*backend.ChordDelivery) (bool, error)) error {
	if err := b.ensureChordIndexes(ctx); err != nil {
		return err
	}
	for attempt := 0; attempt < 16; attempt++ {
		var document mongoChordDocument
		if err := b.chordTable.FindOne(ctx, bson.M{"_id": deliveryKey}).Decode(&document); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return backend.ErrChordNotFound
			}
			return err
		}
		version := document.Version
		changed, err := mutate(&document.ChordDelivery)
		if err != nil || !changed {
			return err
		}
		result, err := b.chordTable.ReplaceOne(ctx, bson.M{"_id": deliveryKey, "version": version}, &document)
		if err != nil {
			return err
		}
		if result.ModifiedCount == 1 {
			return nil
		}
	}
	return errors.New("mongo chord CAS exhausted")
}

func (b *BackendMongoDB) createChordIndexes(ctx context.Context) error {
	zero := int32(0)
	models := []mongo.IndexModel{
		{Keys: bson.D{{Key: "delivery_key", Value: 1}}, Options: options.Index().SetName("gkit_chord_delivery_key").SetUnique(true)},
		{Keys: bson.D{{Key: "group_id", Value: 1}}, Options: options.Index().SetName("gkit_chord_group_id").SetUnique(true)},
		{Keys: bson.D{{Key: "terminal_expire_at", Value: 1}}, Options: options.Index().SetName("gkit_chord_terminal_expiry").SetExpireAfterSeconds(zero)},
		{Keys: bson.D{{Key: "callback_state", Value: 1}, {Key: "callback_next_attempt_at", Value: 1}}, Options: options.Index().SetName("gkit_chord_due")},
	}
	if _, err := b.chordTable.Indexes().CreateMany(ctx, models); err != nil {
		return fmt.Errorf("backend_mongodb: create chord indexes: %w", err)
	}
	return nil
}

func (b *BackendMongoDB) ensureChordIndexes(ctx context.Context) error {
	b.chordIndexOnce.Do(func() {
		b.chordIndexErr = b.createChordIndexes(ctx)
	})
	return b.chordIndexErr
}
