package backend_db

import (
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/songzhibin97/gkit/distributed/backend"
	"gorm.io/gorm"

	"github.com/songzhibin97/gkit/distributed/task"
)

func newSQLiteBackend(t *testing.T) *BackendSQLDB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	b := &BackendSQLDB{gClient: gdb, resultExpire: 0}
	if err := b.autoMigrate(); err != nil {
		t.Fatalf("auto migrate: %v", err)
	}
	return b
}

type legacyGroupMeta struct {
	ID      uint   `gorm:"column:_id;primarykey"`
	GroupID string `gorm:"column:id;size:191;index"`
}

func (legacyGroupMeta) TableName() string {
	return "group_meta"
}

// TestUpsertStatus_DedupsByTaskID is the regression guard for the broken
// OnConflict target. Repeated SetStateX for the same TaskID must upsert a
// single row to the latest status. The bug (conflict column "task_id", which
// does not exist, on a non-unique index) either errored on the upsert or
// inserted duplicate rows. sqlite's ON CONFLICT requires both the real column
// name and a UNIQUE index, so it fails fast on either regression.
func TestUpsertStatus_DedupsByTaskID(t *testing.T) {
	b := newSQLiteBackend(t)
	sig := &task.Signature{ID: "task-1", GroupID: "g1", Name: "n"}

	if err := b.SetStatePending(sig); err != nil {
		t.Fatalf("SetStatePending: %v", err)
	}
	if err := b.SetStateStarted(sig); err != nil {
		t.Fatalf("SetStateStarted: %v", err)
	}
	if err := b.SetStateSuccess(sig, nil); err != nil {
		t.Fatalf("SetStateSuccess: %v", err)
	}

	var count int64
	if err := b.gClient.Model(&task.Status{}).Where("id = ?", "task-1").Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("rows for task-1 = %d, want 1 (upsert must dedupe by TaskID)", count)
	}

	st, err := b.GetStatus("task-1")
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if st.Status != task.StateSuccess {
		t.Fatalf("status = %v, want StateSuccess (last write wins)", st.Status)
	}
}

// TestUpsertStatus_DistinctTaskIDsCoexist ensures different task IDs each get
// their own row (the unique index is scoped to the task id, not over-broad).
func TestUpsertStatus_DistinctTaskIDsCoexist(t *testing.T) {
	b := newSQLiteBackend(t)
	for _, id := range []string{"a", "b", "c"} {
		if err := b.SetStatePending(&task.Signature{ID: id, GroupID: "g", Name: "n"}); err != nil {
			t.Fatalf("SetStatePending(%s): %v", id, err)
		}
	}
	var count int64
	if err := b.gClient.Model(&task.Status{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Fatalf("rows = %d, want 3 (distinct task IDs must coexist)", count)
	}
}

// TestResetTask_AllowsReuse guards the soft-delete interaction: ResetTask must
// hard-delete so a task ID can be reused afterwards. With a soft delete, the
// unique-index upsert would update the soft-deleted row without clearing
// deleted_at, and GetStatus (which respects deleted_at) could never see it.
func TestResetTask_AllowsReuse(t *testing.T) {
	b := newSQLiteBackend(t)
	sig := &task.Signature{ID: "task-1", GroupID: "g1", Name: "n"}

	if err := b.SetStateSuccess(sig, nil); err != nil {
		t.Fatalf("SetStateSuccess: %v", err)
	}
	if err := b.ResetTask("task-1"); err != nil {
		t.Fatalf("ResetTask: %v", err)
	}
	// Reuse the same task ID after reset.
	if err := b.SetStatePending(sig); err != nil {
		t.Fatalf("SetStatePending after reset: %v", err)
	}
	st, err := b.GetStatus("task-1")
	if err != nil {
		t.Fatalf("GetStatus after reset+reuse: %v (a soft delete would hide the row)", err)
	}
	if st.Status != task.StatePending {
		t.Fatalf("status = %v, want StatePending after reuse", st.Status)
	}
}

func TestGroupTakeOverRejectsDuplicateGroupID(t *testing.T) {
	b := newSQLiteBackend(t)
	const groupID = "duplicate-group"

	if err := b.GroupTakeOver(groupID, "first", "task-1"); err != nil {
		t.Fatalf("first GroupTakeOver: %v", err)
	}
	triggered, err := b.TriggerCompleted(groupID)
	if err != nil || !triggered {
		t.Fatalf("first TriggerCompleted = (%v, %v), want (true, nil)", triggered, err)
	}

	err = b.GroupTakeOver(groupID, "second", "task-2")
	if !errors.Is(err, backend.ErrGroupAlreadyExists) {
		t.Errorf("second GroupTakeOver error = %v, want ErrGroupAlreadyExists", err)
	} else if !strings.Contains(err.Error(), groupID) {
		t.Errorf("second GroupTakeOver error = %q, want group ID context", err)
	}

	var count int64
	if err := b.gClient.Model(&task.GroupMeta{}).Where("id = ?", groupID).Count(&count).Error; err != nil {
		t.Fatalf("count groups: %v", err)
	}
	if count != 1 {
		t.Errorf("rows for %q = %d, want 1", groupID, count)
	}

	triggered, err = b.TriggerCompleted(groupID)
	if err != nil {
		t.Fatalf("second TriggerCompleted: %v", err)
	}
	if triggered {
		t.Error("second TriggerCompleted succeeded after duplicate takeover")
	}
}

func TestAutoMigrateRejectsHistoricalDuplicateGroupIDsWithoutDeleting(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := gdb.AutoMigrate(&legacyGroupMeta{}); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	for index := 0; index < 2; index++ {
		if err := gdb.Create(&legacyGroupMeta{GroupID: "duplicate-group"}).Error; err != nil {
			t.Fatalf("insert legacy duplicate %d: %v", index, err)
		}
	}

	b := &BackendSQLDB{gClient: gdb}
	if err := b.autoMigrate(); err == nil {
		t.Fatal("autoMigrate succeeded with historical duplicate group IDs")
	}

	var count int64
	if err := gdb.Table("group_meta").Where("id = ?", "duplicate-group").Count(&count).Error; err != nil {
		t.Fatalf("count historical groups: %v", err)
	}
	if count != 2 {
		t.Fatalf("historical duplicate rows after failed migration = %d, want 2", count)
	}
}

// TestNewBackendSQLDB_NilOnFailure pins the deprecated constructor's documented
// contract: it returns nil (not panic) on failure. An unsupported dbType fails
// before any connection, so this needs no live DB. (Regression: the PR briefly
// made it panic, which crashed callers — and tests — that check for nil.)
func TestNewBackendSQLDB_NilOnFailure(t *testing.T) {
	if b := NewBackendSQLDB(nil, -1, "no-such-db", nil); b != nil {
		t.Fatal("NewBackendSQLDB with an unsupported dbType should return nil, not panic")
	}
}

// TestNewBackendSQLDBE_ErrorOnFailure pins the error-returning variant.
func TestNewBackendSQLDBE_ErrorOnFailure(t *testing.T) {
	if _, err := NewBackendSQLDBE(nil, -1, "no-such-db", nil); err == nil {
		t.Fatal("NewBackendSQLDBE with an unsupported dbType should return an error")
	}
}
