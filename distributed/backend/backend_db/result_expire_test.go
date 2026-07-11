package backend_db

import (
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/gorm"
)

type sqlExpiryClock struct {
	mu  sync.RWMutex
	now time.Time
}

func (c *sqlExpiryClock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
}

func (c *sqlExpiryClock) Set(now time.Time) {
	c.mu.Lock()
	c.now = now
	c.mu.Unlock()
}

func newSQLiteExpiryBackend(t *testing.T, now time.Time, expire int64) (*BackendSQLDB, *sqlExpiryClock) {
	t.Helper()
	b := newSQLiteBackend(t)
	sqlDB, err := b.gClient.DB()
	if err != nil {
		t.Fatalf("get sqlite connection: %v", err)
	}
	// SQLite :memory: databases are connection-local. Keep this deterministic
	// when race tests issue concurrent backend calls.
	sqlDB.SetMaxOpenConns(1)
	clock := &sqlExpiryClock{now: now}
	b.now = clock.Now
	b.SetResultExpire(expire)
	return b, clock
}

func countUnscopedRows(t *testing.T, db *gorm.DB, model interface{}, where string, args ...interface{}) int64 {
	t.Helper()
	var count int64
	if err := db.Unscoped().Model(model).Where(where, args...).Count(&count).Error; err != nil {
		t.Fatalf("count rows: %v", err)
	}
	return count
}

func readStoredStatus(t *testing.T, b *BackendSQLDB, taskID string) task.Status {
	t.Helper()
	var status task.Status
	if err := b.gClient.Where("id = ?", taskID).First(&status).Error; err != nil {
		t.Fatalf("read stored status: %v", err)
	}
	return status
}

func TestSQLResultExpireBoundaryAndModes(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 123, time.UTC)
	signature := &task.Signature{ID: "task", GroupID: "group", Name: "task"}

	t.Run("positive expires at exact boundary", func(t *testing.T) {
		b, clock := newSQLiteExpiryBackend(t, base, 10)
		if err := b.SetStatePending(signature); err != nil {
			t.Fatalf("SetStatePending: %v", err)
		}
		stored := readStoredStatus(t, b, signature.ID)
		if stored.TTL != 10 || !stored.CreateAt.Equal(base) {
			t.Fatalf("stored retention = (%d, %v), want (10, %v)", stored.TTL, stored.CreateAt, base)
		}

		clock.Set(base.Add(10*time.Second - time.Nanosecond))
		if _, err := b.GetStatus(signature.ID); err != nil {
			t.Fatalf("GetStatus immediately before expiry: %v", err)
		}
		clock.Set(base.Add(10 * time.Second))
		status, err := b.GetStatus(signature.ID)
		if !errors.Is(err, gorm.ErrRecordNotFound) || status != nil {
			t.Fatalf("GetStatus at expiry = (%v, %v), want (nil, gorm.ErrRecordNotFound)", status, err)
		}
		if count := countUnscopedRows(t, b.gClient, &task.Status{}, "_id = ?", stored.ID); count != 0 {
			t.Fatalf("physical expired rows = %d, want 0", count)
		}
	})

	t.Run("zero uses one hour default", func(t *testing.T) {
		b, clock := newSQLiteExpiryBackend(t, base, 0)
		if err := b.SetStatePending(signature); err != nil {
			t.Fatalf("SetStatePending: %v", err)
		}
		stored := readStoredStatus(t, b, signature.ID)
		if stored.TTL != defaultSQLResultExpireSeconds {
			t.Fatalf("stored TTL = %d, want %d", stored.TTL, defaultSQLResultExpireSeconds)
		}
		clock.Set(base.Add(time.Hour - time.Nanosecond))
		if _, err := b.GetStatus(signature.ID); err != nil {
			t.Fatalf("GetStatus immediately before default expiry: %v", err)
		}
		clock.Set(base.Add(time.Hour))
		if _, err := b.GetStatus(signature.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("GetStatus at default expiry error = %v, want gorm.ErrRecordNotFound", err)
		}
	})

	t.Run("group zero uses one hour default", func(t *testing.T) {
		b, clock := newSQLiteExpiryBackend(t, base, 0)
		if err := b.GroupTakeOver("group", "group"); err != nil {
			t.Fatalf("GroupTakeOver: %v", err)
		}
		var stored task.GroupMeta
		if err := b.gClient.Where("id = ?", "group").First(&stored).Error; err != nil {
			t.Fatalf("read stored group: %v", err)
		}
		if stored.TTL != defaultSQLResultExpireSeconds || !stored.CreateAt.Equal(base) {
			t.Fatalf("stored group retention = (%d, %v), want (%d, %v)", stored.TTL, stored.CreateAt, defaultSQLResultExpireSeconds, base)
		}
		clock.Set(base.Add(time.Hour))
		if _, err := b.GroupTaskStatus(stored.GroupID); !errors.Is(err, gorm.ErrRecordNotFound) {
			t.Fatalf("GroupTaskStatus at default expiry error = %v, want gorm.ErrRecordNotFound", err)
		}
	})

	t.Run("negative never expires", func(t *testing.T) {
		b, clock := newSQLiteExpiryBackend(t, base, -7)
		if err := b.SetStatePending(signature); err != nil {
			t.Fatalf("SetStatePending: %v", err)
		}
		clock.Set(base.Add(100 * 365 * 24 * time.Hour))
		status, err := b.GetStatus(signature.ID)
		if err != nil || status == nil {
			t.Fatalf("GetStatus with negative TTL = (%v, %v), want live row", status, err)
		}
	})

	t.Run("large TTL does not overflow", func(t *testing.T) {
		b, clock := newSQLiteExpiryBackend(t, base, math.MaxInt64)
		if err := b.SetStatePending(signature); err != nil {
			t.Fatalf("SetStatePending: %v", err)
		}
		clock.Set(base.Add(100 * 365 * 24 * time.Hour))
		if _, err := b.GetStatus(signature.ID); err != nil {
			t.Fatalf("GetStatus with large TTL: %v", err)
		}
	})
}

func TestSQLResultExpireStateTransitionsDoNotRestartRetention(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	b, clock := newSQLiteExpiryBackend(t, base, 10)
	signature := &task.Signature{ID: "task", GroupID: "group", Name: "task"}
	if err := b.SetStatePending(signature); err != nil {
		t.Fatalf("SetStatePending: %v", err)
	}
	first := readStoredStatus(t, b, signature.ID)

	clock.Set(base.Add(5 * time.Second))
	if err := b.SetStateSuccess(signature, nil); err != nil {
		t.Fatalf("SetStateSuccess: %v", err)
	}
	updated := readStoredStatus(t, b, signature.ID)
	if updated.ID != first.ID || updated.TTL != first.TTL || !updated.CreateAt.Equal(first.CreateAt) {
		t.Fatalf("state transition changed retention: before=%+v after=%+v", first, updated)
	}
	clock.Set(base.Add(10 * time.Second))
	if _, err := b.GetStatus(signature.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("GetStatus at original expiry error = %v, want gorm.ErrRecordNotFound", err)
	}
}

func TestSQLResultExpireGroupReadAndTransitionSeams(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name  string
		touch func(*BackendSQLDB, string) error
	}{
		{name: "GroupTaskStatus", touch: func(b *BackendSQLDB, groupID string) error {
			_, err := b.GroupTaskStatus(groupID)
			return err
		}},
		{name: "GroupCompleted", touch: func(b *BackendSQLDB, groupID string) error {
			_, err := b.GroupCompleted(groupID)
			return err
		}},
		{name: "TriggerCompleted", touch: func(b *BackendSQLDB, groupID string) error {
			_, err := b.TriggerCompleted(groupID)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, clock := newSQLiteExpiryBackend(t, base, 5)
			if err := b.GroupTakeOver("group", "group"); err != nil {
				t.Fatalf("GroupTakeOver: %v", err)
			}
			var stored task.GroupMeta
			if err := b.gClient.Where("id = ?", "group").First(&stored).Error; err != nil {
				t.Fatalf("read stored group: %v", err)
			}
			clock.Set(base.Add(5 * time.Second))
			if err := tt.touch(b, stored.GroupID); !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("touch error = %v, want gorm.ErrRecordNotFound", err)
			}
			if count := countUnscopedRows(t, b.gClient, &task.GroupMeta{}, "_id = ?", stored.ID); count != 0 {
				t.Fatalf("physical expired group rows = %d, want 0", count)
			}
		})
	}

}

func TestSQLResultExpireBatchStatusOmitsExpiredMembers(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	b, clock := newSQLiteExpiryBackend(t, base, 5)
	if err := b.SetStatePending(&task.Signature{ID: "member", GroupID: "group", Name: "task"}); err != nil {
		t.Fatalf("SetStatePending: %v", err)
	}
	stored := readStoredStatus(t, b, "member")

	clock.Set(base.Add(6 * time.Second))
	statuses, err := b.getTaskStatus([]string{"member"})
	if err != nil {
		t.Fatalf("getTaskStatus: %v", err)
	}
	if len(statuses) != 0 {
		t.Fatalf("statuses = %v, want expired member omitted", statuses)
	}
	if count := countUnscopedRows(t, b.gClient, &task.Status{}, "_id = ?", stored.ID); count != 0 {
		t.Fatalf("physical expired member rows = %d, want 0", count)
	}
}

func TestSQLResultExpireReusesExpiredIdentifiers(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)

	t.Run("task", func(t *testing.T) {
		b, clock := newSQLiteExpiryBackend(t, base, 5)
		signature := &task.Signature{ID: "task", GroupID: "group", Name: "task"}
		if err := b.SetStatePending(signature); err != nil {
			t.Fatalf("first SetStatePending: %v", err)
		}
		first := readStoredStatus(t, b, signature.ID)
		clock.Set(base.Add(5 * time.Second))
		if err := b.SetStatePending(signature); err != nil {
			t.Fatalf("replacement SetStatePending: %v", err)
		}
		replacement := readStoredStatus(t, b, signature.ID)
		if replacement.ID == first.ID || !replacement.CreateAt.Equal(clock.Now()) {
			t.Fatalf("replacement did not get fresh identity/window: first=%+v replacement=%+v", first, replacement)
		}
		if count := countUnscopedRows(t, b.gClient, &task.Status{}, "id = ?", signature.ID); count != 1 {
			t.Fatalf("physical rows for reused task = %d, want 1", count)
		}
	})

	t.Run("group", func(t *testing.T) {
		b, clock := newSQLiteExpiryBackend(t, base, 5)
		if err := b.GroupTakeOver("group", "first"); err != nil {
			t.Fatalf("first GroupTakeOver: %v", err)
		}
		var first task.GroupMeta
		if err := b.gClient.Where("id = ?", "group").First(&first).Error; err != nil {
			t.Fatalf("read first group: %v", err)
		}
		clock.Set(base.Add(5 * time.Second))
		if err := b.GroupTakeOver("group", "replacement"); err != nil {
			t.Fatalf("replacement GroupTakeOver: %v", err)
		}
		var replacement task.GroupMeta
		if err := b.gClient.Where("id = ?", "group").First(&replacement).Error; err != nil {
			t.Fatalf("read replacement group: %v", err)
		}
		if replacement.ID == first.ID || !replacement.CreateAt.Equal(clock.Now()) {
			t.Fatalf("replacement did not get fresh identity/window: first=%+v replacement=%+v", first, replacement)
		}
		if count := countUnscopedRows(t, b.gClient, &task.GroupMeta{}, "id = ?", "group"); count != 1 {
			t.Fatalf("physical rows for reused group = %d, want 1", count)
		}
	})
}

func TestSQLResultExpireStaleSnapshotsPreserveReplacements(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	now := base.Add(10 * time.Second)
	b, _ := newSQLiteExpiryBackend(t, now, 5)

	t.Run("task", func(t *testing.T) {
		stale := &task.Status{TaskID: "task", TTL: 5, CreateAt: base}
		if err := b.gClient.Create(stale).Error; err != nil {
			t.Fatalf("create stale status: %v", err)
		}
		if err := b.gClient.Unscoped().Where("_id = ?", stale.ID).Delete(&task.Status{}).Error; err != nil {
			t.Fatalf("replace stale status: %v", err)
		}
		replacement := &task.Status{TaskID: stale.TaskID, TTL: 5, CreateAt: now, Status: task.StatePending}
		if err := b.gClient.Create(replacement).Error; err != nil {
			t.Fatalf("create replacement status: %v", err)
		}
		deleted, err := b.deleteExpiredStatusSnapshot(stale, now)
		if err != nil || deleted {
			t.Fatalf("stale cleanup = (%t, %v), want (false, nil)", deleted, err)
		}
		if count := countUnscopedRows(t, b.gClient, &task.Status{}, "_id = ?", replacement.ID); count != 1 {
			t.Fatalf("replacement status rows = %d, want 1", count)
		}
	})

	t.Run("group", func(t *testing.T) {
		stale := &task.GroupMeta{GroupID: "group", TTL: 5, CreateAt: base}
		if err := b.gClient.Create(stale).Error; err != nil {
			t.Fatalf("create stale group: %v", err)
		}
		if err := b.gClient.Unscoped().Where("_id = ?", stale.ID).Delete(&task.GroupMeta{}).Error; err != nil {
			t.Fatalf("replace stale group: %v", err)
		}
		replacement := &task.GroupMeta{GroupID: stale.GroupID, TTL: 5, CreateAt: now}
		if err := b.gClient.Create(replacement).Error; err != nil {
			t.Fatalf("create replacement group: %v", err)
		}
		deleted, err := b.deleteExpiredGroupSnapshot(stale, now)
		if err != nil || deleted {
			t.Fatalf("stale cleanup = (%t, %v), want (false, nil)", deleted, err)
		}
		if count := countUnscopedRows(t, b.gClient, &task.GroupMeta{}, "_id = ?", replacement.ID); count != 1 {
			t.Fatalf("replacement group rows = %d, want 1", count)
		}
	})
}

func TestSQLResultExpireCleanupErrorsAreReturned(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	cleanupErr := errors.New("cleanup failed")

	tests := []struct {
		name  string
		touch func(*BackendSQLDB, *task.Signature) error
	}{
		{name: "read", touch: func(b *BackendSQLDB, signature *task.Signature) error {
			_, err := b.GetStatus(signature.ID)
			return err
		}},
		{name: "same ID write", touch: func(b *BackendSQLDB, signature *task.Signature) error {
			return b.SetStateStarted(signature)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, clock := newSQLiteExpiryBackend(t, base, 5)
			signature := &task.Signature{ID: "task", GroupID: "group", Name: "task"}
			if err := b.SetStatePending(signature); err != nil {
				t.Fatalf("SetStatePending: %v", err)
			}
			stored := readStoredStatus(t, b, signature.ID)
			if err := b.gClient.Callback().Delete().Before("gorm:delete").Register("test:expiry-delete-failure", func(db *gorm.DB) {
				db.AddError(cleanupErr)
			}); err != nil {
				t.Fatalf("register delete callback: %v", err)
			}
			clock.Set(base.Add(5 * time.Second))
			err := tt.touch(b, signature)
			if !errors.Is(err, cleanupErr) {
				t.Fatalf("touch error = %v, want cleanup failure", err)
			}
			if count := countUnscopedRows(t, b.gClient, &task.Status{}, "_id = ?", stored.ID); count != 1 {
				t.Fatalf("rows after failed cleanup = %d, want original row retained", count)
			}
			after := readStoredStatus(t, b, signature.ID)
			if !reflect.DeepEqual(after, stored) {
				t.Fatalf("stored status changed after failed cleanup:\n before=%+v\n  after=%+v", stored, after)
			}
		})
	}

	t.Run("group read", func(t *testing.T) {
		b, clock := newSQLiteExpiryBackend(t, base, 5)
		if err := b.GroupTakeOver("group", "group"); err != nil {
			t.Fatalf("GroupTakeOver: %v", err)
		}
		var stored task.GroupMeta
		if err := b.gClient.Where("id = ?", "group").First(&stored).Error; err != nil {
			t.Fatalf("read stored group: %v", err)
		}
		if err := b.gClient.Callback().Delete().Before("gorm:delete").Register("test:group-expiry-delete-failure", func(db *gorm.DB) {
			db.AddError(cleanupErr)
		}); err != nil {
			t.Fatalf("register delete callback: %v", err)
		}
		clock.Set(base.Add(5 * time.Second))
		_, err := b.GroupTaskStatus(stored.GroupID)
		if !errors.Is(err, cleanupErr) {
			t.Fatalf("GroupTaskStatus error = %v, want cleanup failure", err)
		}
		if count := countUnscopedRows(t, b.gClient, &task.GroupMeta{}, "_id = ?", stored.ID); count != 1 {
			t.Fatalf("rows after failed group cleanup = %d, want original row retained", count)
		}
	})
}

func TestSQLResultExpireConcurrentConfigurationAndWrites(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	b, _ := newSQLiteExpiryBackend(t, base, 1)
	var wg sync.WaitGroup
	for index := 0; index < 20; index++ {
		index := index
		wg.Add(2)
		go func() {
			defer wg.Done()
			values := []int64{0, -1, 1, 60}
			b.SetResultExpire(values[index%len(values)])
		}()
		go func() {
			defer wg.Done()
			signature := &task.Signature{ID: string(rune('a' + index)), GroupID: "group", Name: "task"}
			if err := b.SetStatePending(signature); err != nil {
				t.Errorf("SetStatePending(%d): %v", index, err)
			}
		}()
	}
	wg.Wait()
}
