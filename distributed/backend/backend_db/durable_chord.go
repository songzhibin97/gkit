package backend_db

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var _ backend.DurableChordBackend = (*BackendSQLDB)(nil)

type chordDeliveryModel struct {
	DeliveryKey         string     `gorm:"column:delivery_key;primaryKey;size:160"`
	GroupID             string     `gorm:"column:group_id;uniqueIndex:uq_chord_delivery_group;size:255"`
	DefinitionHash      string     `gorm:"column:definition_hash;size:64;not null"`
	RegistrationOwner   string     `gorm:"column:registration_owner;size:64;not null"`
	RegistrationVersion int64      `gorm:"column:registration_version;not null"`
	Version             int64      `gorm:"column:version;not null"`
	State               string     `gorm:"column:state;index:idx_chord_delivery_state;size:32;not null"`
	TerminalAt          *time.Time `gorm:"column:terminal_at"`
	TerminalExpireAt    *time.Time `gorm:"column:terminal_expire_at;index:idx_chord_terminal_expire"`
	Record              []byte     `gorm:"column:record;not null"`
}

func (chordDeliveryModel) TableName() string { return "chord_deliveries" }

type chordMemberPublicationModel struct {
	ID          uint       `gorm:"primaryKey"`
	DeliveryKey string     `gorm:"column:delivery_key;uniqueIndex:uq_chord_member_publication,priority:1;index;size:160"`
	Ordinal     int        `gorm:"column:ordinal;uniqueIndex:uq_chord_member_publication,priority:2"`
	TaskID      string     `gorm:"column:task_id;size:255;not null"`
	Payload     []byte     `gorm:"column:payload;not null"`
	State       string     `gorm:"column:state;index:idx_chord_member_state;size:32;not null"`
	Version     int64      `gorm:"column:version;not null"`
	LeaseOwner  string     `gorm:"column:lease_owner;size:64"`
	LeaseExpiry *time.Time `gorm:"column:lease_expiry"`
}

func (chordMemberPublicationModel) TableName() string { return "chord_member_publications" }

type chordMemberReceiptModel struct {
	ID          uint   `gorm:"primaryKey"`
	DeliveryKey string `gorm:"column:delivery_key;uniqueIndex:uq_chord_member_receipt,priority:1;index;size:160"`
	Ordinal     int    `gorm:"column:ordinal;uniqueIndex:uq_chord_member_receipt,priority:2"`
	TaskID      string `gorm:"column:task_id;size:255;not null"`
	Outcome     string `gorm:"column:outcome;size:16;not null"`
	Results     []byte `gorm:"column:results"`
}

func (chordMemberReceiptModel) TableName() string { return "chord_member_receipts" }

func (b *BackendSQLDB) RegisterChord(ctx context.Context, registration backend.ChordRegistration) (backend.ChordRegistrationRef, error) {
	if err := backend.FinalizeChordRegistration(&registration); err != nil {
		return backend.ChordRegistrationRef{}, err
	}
	owner, err := backend.NewChordOwner()
	if err != nil {
		return backend.ChordRegistrationRef{}, err
	}
	delivery := backend.NewChordDelivery(registration, owner, time.Now())
	for attempt := 0; attempt < 8; attempt++ {
		err = b.gClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var existing chordDeliveryModel
			findErr := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("group_id = ?", registration.GroupID).First(&existing).Error
			if findErr == nil {
				var current backend.ChordDelivery
				if err := json.Unmarshal(existing.Record, &current); err != nil {
					return err
				}
				if !backend.ChordRegistrationMatches(&current, registration) {
					return backend.ErrChordRegistrationConflict
				}
				delivery = current
				return nil
			}
			if !errors.Is(findErr, gorm.ErrRecordNotFound) {
				return findErr
			}
			model, err := makeChordDeliveryModel(&delivery)
			if err != nil {
				return err
			}
			if err := tx.Create(&model).Error; err != nil {
				return err
			}
			return syncChordMemberRows(tx, &delivery)
		})
		if err == nil {
			break
		}
		if errors.Is(err, backend.ErrChordRegistrationConflict) {
			return backend.ChordRegistrationRef{}, err
		}
		// A concurrent identical registration can win the unique group-ID
		// insert after our initial missing-row read. Resolve that race by
		// attaching to the committed winner. If its transaction has not yet
		// committed, retry the bounded registration operation.
		var existing chordDeliveryModel
		if findErr := b.gClient.WithContext(ctx).Where("group_id = ?", registration.GroupID).First(&existing).Error; findErr == nil {
			var current backend.ChordDelivery
			if decodeErr := json.Unmarshal(existing.Record, &current); decodeErr != nil {
				return backend.ChordRegistrationRef{}, decodeErr
			}
			if !backend.ChordRegistrationMatches(&current, registration) {
				return backend.ChordRegistrationRef{}, backend.ErrChordRegistrationConflict
			}
			return backend.ChordRegistrationRef{DeliveryKey: current.DeliveryKey, Owner: current.RegistrationOwner, Version: current.RegistrationVersion, Created: false}, nil
		}
		if attempt == 7 {
			return backend.ChordRegistrationRef{}, err
		}
		timer := time.NewTimer(time.Duration(attempt+1) * 2 * time.Millisecond)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				select {
				case <-timer.C:
				default:
				}
			}
			return backend.ChordRegistrationRef{}, ctx.Err()
		case <-timer.C:
		}
	}
	created := delivery.RegistrationOwner == owner
	return backend.ChordRegistrationRef{DeliveryKey: delivery.DeliveryKey, Owner: delivery.RegistrationOwner, Version: delivery.RegistrationVersion, Created: created}, nil
}

func (b *BackendSQLDB) AbortRegistration(ctx context.Context, ref backend.ChordRegistrationRef) error {
	if !ref.Created {
		return backend.ErrChordRegistrationOwnershipLost
	}
	return b.gClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var model chordDeliveryModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("delivery_key = ?", ref.DeliveryKey).First(&model).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil
			}
			return err
		}
		var delivery backend.ChordDelivery
		if err := json.Unmarshal(model.Record, &delivery); err != nil {
			return err
		}
		if delivery.RegistrationOwner != ref.Owner || delivery.RegistrationVersion != ref.Version {
			return backend.ErrChordRegistrationOwnershipLost
		}
		if delivery.MemberPublicationStarted {
			return backend.ErrChordPublicationStarted
		}
		if err := tx.Where("delivery_key = ?", ref.DeliveryKey).Delete(&chordMemberReceiptModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("delivery_key = ?", ref.DeliveryKey).Delete(&chordMemberPublicationModel{}).Error; err != nil {
			return err
		}
		return tx.Where("delivery_key = ?", ref.DeliveryKey).Delete(&chordDeliveryModel{}).Error
	})
}

func (b *BackendSQLDB) ClaimMemberPublication(ctx context.Context, claim backend.ChordMemberClaim) (lease backend.ChordMemberLease, claimed bool, err error) {
	err = b.updateChordDelivery(ctx, claim.DeliveryKey, func(delivery *backend.ChordDelivery) (bool, error) {
		var transitionErr error
		lease, claimed, transitionErr = backend.ClaimChordMember(delivery, claim)
		return claimed, transitionErr
	})
	return lease, claimed, err
}

func (b *BackendSQLDB) RecordMemberPublishOutcome(ctx context.Context, lease backend.ChordMemberLease, outcome backend.ChordPublishOutcome) error {
	return b.updateChordDelivery(ctx, lease.DeliveryKey, func(delivery *backend.ChordDelivery) (bool, error) {
		if err := backend.ApplyChordMemberPublishOutcome(delivery, lease, outcome); err != nil {
			return false, err
		}
		return true, nil
	})
}

func (b *BackendSQLDB) RecordMemberTerminal(ctx context.Context, deliveryKey string, ordinal int, taskID string, outcome backend.MemberTerminalOutcome, results []*task.Result) error {
	return b.updateChordDelivery(ctx, deliveryKey, func(delivery *backend.ChordDelivery) (bool, error) {
		before := delivery.Version
		if err := backend.ApplyChordMemberTerminal(delivery, ordinal, taskID, outcome, results, time.Now()); err != nil {
			return false, err
		}
		return delivery.Version != before, nil
	})
}

func (b *BackendSQLDB) ScanChordDeliveries(ctx context.Context, scan backend.ChordScan) (backend.ChordDeliveryPage, error) {
	limit := scan.Limit
	if limit <= 0 {
		limit = 100
	}
	query := b.gClient.WithContext(ctx).Order("delivery_key ASC").Limit(limit + 1)
	if scan.Cursor != "" {
		query = query.Where("delivery_key > ?", scan.Cursor)
	}
	var models []chordDeliveryModel
	if err := query.Find(&models).Error; err != nil {
		return backend.ChordDeliveryPage{}, err
	}
	page := backend.ChordDeliveryPage{}
	for _, model := range models {
		var delivery backend.ChordDelivery
		if err := json.Unmarshal(model.Record, &delivery); err != nil {
			return backend.ChordDeliveryPage{}, err
		}
		page.Deliveries = append(page.Deliveries, delivery)
	}
	if len(page.Deliveries) > limit {
		page.Deliveries = page.Deliveries[:limit]
		page.NextCursor = page.Deliveries[len(page.Deliveries)-1].DeliveryKey
	}
	return page, nil
}

func (b *BackendSQLDB) ReconcileChord(ctx context.Context, deliveryKey string) error {
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
	if err := b.gClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		ids := make([]string, len(delivery.Members))
		for index := range delivery.Members {
			ids[index] = delivery.Members[index].TaskID
		}
		var group task.GroupMeta
		groupErr := tx.Where("id = ?", delivery.GroupID).First(&group).Error
		if errors.Is(groupErr, gorm.ErrRecordNotFound) {
			group = *task.InitGroupMeta(delivery.GroupID, delivery.GroupName, b.resultExpire, ids...)
			if err := tx.Create(&group).Error; err != nil {
				return err
			}
		} else if groupErr != nil {
			return groupErr
		} else if !sameChordStrings([]string(group.TaskIDs), ids) {
			return backend.ErrChordRegistrationConflict
		}
		for index := range delivery.Members {
			var signature task.Signature
			if err := json.Unmarshal(delivery.Members[index].Payload, &signature); err != nil {
				return err
			}
			var existing task.Status
			statusErr := tx.Where("id = ?", signature.ID).First(&existing).Error
			if errors.Is(statusErr, gorm.ErrRecordNotFound) {
				if err := tx.Create(task.NewPendingState(&signature)).Error; err != nil {
					return err
				}
			} else if statusErr != nil {
				return statusErr
			} else if existing.GroupID != delivery.GroupID {
				return backend.ErrChordRegistrationConflict
			}
		}
		return nil
	}); err != nil {
		return err
	}
	// State publication is intentionally done after setup transaction commits.
	return b.markChordMembersReady(ctx, deliveryKey)
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

func (b *BackendSQLDB) markChordMembersReady(ctx context.Context, deliveryKey string) error {
	return b.updateChordDelivery(ctx, deliveryKey, func(delivery *backend.ChordDelivery) (bool, error) {
		backend.PrepareChordMembers(delivery, time.Now())
		return true, nil
	})
}

func (b *BackendSQLDB) ClaimCallbackPublication(ctx context.Context, claim backend.ChordCallbackClaim) (lease backend.ChordCallbackLease, claimed bool, err error) {
	delivery, err := b.getChordDelivery(ctx, claim.DeliveryKey)
	if err != nil {
		return lease, false, err
	}
	if len(delivery.CallbackPayload) > 0 {
		var callback task.Signature
		if err := json.Unmarshal(delivery.CallbackPayload, &callback); err != nil {
			return lease, false, err
		}
		if err := b.gClient.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(task.NewPendingState(&callback)).Error; err != nil {
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

func (b *BackendSQLDB) RecordCallbackPublishOutcome(ctx context.Context, lease backend.ChordCallbackLease, outcome backend.ChordPublishOutcome) error {
	return b.updateChordDelivery(ctx, lease.DeliveryKey, func(delivery *backend.ChordDelivery) (bool, error) {
		if err := backend.ApplyChordCallbackPublishOutcome(delivery, lease, outcome); err != nil {
			return false, err
		}
		return true, nil
	})
}

func (b *BackendSQLDB) RecordCallbackTerminal(ctx context.Context, deliveryKey string, outcome backend.CallbackTerminalOutcome) error {
	return b.updateChordDelivery(ctx, deliveryKey, func(delivery *backend.ChordDelivery) (bool, error) {
		before := delivery.Version
		if err := backend.ApplyChordCallbackTerminal(delivery, outcome, time.Now()); err != nil {
			return false, err
		}
		return delivery.Version != before, nil
	})
}

func (b *BackendSQLDB) CleanupTerminalChordDeliveries(ctx context.Context, now time.Time, limit int) (int, error) {
	if limit <= 0 {
		limit = 100
	}
	var keys []string
	if err := b.gClient.WithContext(ctx).Model(&chordDeliveryModel{}).
		Where("terminal_expire_at IS NOT NULL AND terminal_expire_at <= ?", now).
		Order("terminal_expire_at ASC").Limit(limit).Pluck("delivery_key", &keys).Error; err != nil {
		return 0, err
	}
	if len(keys) == 0 {
		return 0, nil
	}
	err := b.gClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("delivery_key IN ?", keys).Delete(&chordMemberReceiptModel{}).Error; err != nil {
			return err
		}
		if err := tx.Where("delivery_key IN ?", keys).Delete(&chordMemberPublicationModel{}).Error; err != nil {
			return err
		}
		return tx.Where("delivery_key IN ?", keys).Delete(&chordDeliveryModel{}).Error
	})
	return len(keys), err
}

func (b *BackendSQLDB) getChordDelivery(ctx context.Context, deliveryKey string) (*backend.ChordDelivery, error) {
	var model chordDeliveryModel
	if err := b.gClient.WithContext(ctx).Where("delivery_key = ?", deliveryKey).First(&model).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, backend.ErrChordNotFound
		}
		return nil, err
	}
	var delivery backend.ChordDelivery
	if err := json.Unmarshal(model.Record, &delivery); err != nil {
		return nil, err
	}
	return &delivery, nil
}

func (b *BackendSQLDB) updateChordDelivery(ctx context.Context, deliveryKey string, mutate func(*backend.ChordDelivery) (bool, error)) error {
	for attempt := 0; attempt < 16; attempt++ {
		retry := false
		err := b.gClient.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			var model chordDeliveryModel
			if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("delivery_key = ?", deliveryKey).First(&model).Error; err != nil {
				if errors.Is(err, gorm.ErrRecordNotFound) {
					return backend.ErrChordNotFound
				}
				return err
			}
			var delivery backend.ChordDelivery
			if err := json.Unmarshal(model.Record, &delivery); err != nil {
				return err
			}
			changed, err := mutate(&delivery)
			if err != nil || !changed {
				return err
			}
			updated, err := makeChordDeliveryModel(&delivery)
			if err != nil {
				return err
			}
			result := tx.Model(&chordDeliveryModel{}).Where("delivery_key = ? AND version = ?", deliveryKey, model.Version).Updates(map[string]interface{}{
				"definition_hash":      updated.DefinitionHash,
				"registration_owner":   updated.RegistrationOwner,
				"registration_version": updated.RegistrationVersion,
				"version":              updated.Version,
				"state":                updated.State,
				"terminal_at":          updated.TerminalAt,
				"terminal_expire_at":   updated.TerminalExpireAt,
				"record":               updated.Record,
			})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				retry = true
				return nil
			}
			return syncChordMemberRows(tx, &delivery)
		})
		if err != nil {
			return err
		}
		if !retry {
			return nil
		}
	}
	return errors.New("sql chord CAS exhausted")
}

func makeChordDeliveryModel(delivery *backend.ChordDelivery) (chordDeliveryModel, error) {
	body, err := json.Marshal(delivery)
	if err != nil {
		return chordDeliveryModel{}, err
	}
	var terminalAt *time.Time
	if !delivery.TerminalAt.IsZero() {
		value := delivery.TerminalAt
		terminalAt = &value
	}
	return chordDeliveryModel{
		DeliveryKey:         delivery.DeliveryKey,
		GroupID:             delivery.GroupID,
		DefinitionHash:      delivery.DefinitionHash,
		RegistrationOwner:   delivery.RegistrationOwner,
		RegistrationVersion: delivery.RegistrationVersion,
		Version:             delivery.Version,
		State:               string(delivery.CallbackState),
		TerminalAt:          terminalAt,
		TerminalExpireAt:    delivery.TerminalExpireAt,
		Record:              body,
	}, nil
}

func syncChordMemberRows(tx *gorm.DB, delivery *backend.ChordDelivery) error {
	for index := range delivery.Members {
		member := &delivery.Members[index]
		var leaseExpiry *time.Time
		if !member.LeaseExpiresAt.IsZero() {
			value := member.LeaseExpiresAt
			leaseExpiry = &value
		}
		publication := chordMemberPublicationModel{
			DeliveryKey: delivery.DeliveryKey,
			Ordinal:     member.Ordinal,
			TaskID:      member.TaskID,
			Payload:     member.Payload,
			State:       string(member.State),
			Version:     member.Version,
			LeaseOwner:  member.LeaseOwner,
			LeaseExpiry: leaseExpiry,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "delivery_key"}, {Name: "ordinal"}},
			DoUpdates: clause.AssignmentColumns([]string{"task_id", "payload", "state", "version", "lease_owner", "lease_expiry"}),
		}).Create(&publication).Error; err != nil {
			return err
		}
		if member.Receipt != nil {
			results, err := json.Marshal(member.Receipt.Results)
			if err != nil {
				return err
			}
			receipt := chordMemberReceiptModel{DeliveryKey: delivery.DeliveryKey, Ordinal: member.Ordinal, TaskID: member.TaskID, Outcome: string(member.Receipt.Outcome), Results: results}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "delivery_key"}, {Name: "ordinal"}},
				DoUpdates: clause.AssignmentColumns([]string{"task_id", "outcome", "results"}),
			}).Create(&receipt).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *BackendSQLDB) migrateDurableChord() error {
	return b.gClient.AutoMigrate(&chordDeliveryModel{}, &chordMemberPublicationModel{}, &chordMemberReceiptModel{})
}
