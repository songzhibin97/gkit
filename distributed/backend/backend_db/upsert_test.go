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

type publicationAttemptBackendForTest interface {
	SetStatePendingAttempt(*task.Signature, string) error
	FailPendingAttempt(*task.Signature, string, string) (bool, error)
}

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

func TestNonFailureTransitionsClearPreviousError(t *testing.T) {
	tests := []struct {
		name       string
		wantState  task.State
		transition func(*BackendSQLDB, *task.Signature) error
	}{
		{name: "pending", wantState: task.StatePending, transition: func(b *BackendSQLDB, sig *task.Signature) error { return b.SetStatePending(sig) }},
		{name: "received", wantState: task.StateReceived, transition: func(b *BackendSQLDB, sig *task.Signature) error { return b.SetStateReceived(sig) }},
		{name: "started", wantState: task.StateStarted, transition: func(b *BackendSQLDB, sig *task.Signature) error { return b.SetStateStarted(sig) }},
		{name: "retry", wantState: task.StateRetry, transition: func(b *BackendSQLDB, sig *task.Signature) error { return b.SetStateRetry(sig) }},
		{name: "success", wantState: task.StateSuccess, transition: func(b *BackendSQLDB, sig *task.Signature) error { return b.SetStateSuccess(sig, nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := newSQLiteBackend(t)
			sig := &task.Signature{ID: "task-" + tt.name, GroupID: "g1", Name: "n"}
			if err := b.SetStateFailure(sig, "stale failure"); err != nil {
				t.Fatalf("SetStateFailure: %v", err)
			}
			failed, err := b.GetStatus(sig.ID)
			if err != nil {
				t.Fatalf("GetStatus after failure: %v", err)
			}
			if failed.Status != task.StateFailure || failed.Error != "stale failure" {
				t.Fatalf("failure state = (%s, %q), want (FAILURE, stale failure)", failed.Status, failed.Error)
			}
			if err := tt.transition(b, sig); err != nil {
				t.Fatalf("%s transition: %v", tt.name, err)
			}
			status, err := b.GetStatus(sig.ID)
			if err != nil {
				t.Fatalf("GetStatus: %v", err)
			}
			if status.Status != tt.wantState {
				t.Fatalf("status = %s, want %s", status.Status, tt.wantState)
			}
			if status.Error != "" {
				t.Fatalf("%s error = %q, want cleared", tt.name, status.Error)
			}
		})
	}
}

func TestPublicationAttemptCompensation(t *testing.T) {
	b := newSQLiteBackend(t)
	attemptBackend, ok := interface{}(b).(publicationAttemptBackendForTest)
	if !ok {
		t.Fatal("BackendSQLDB does not implement atomic publication-attempt compensation")
	}
	signature := &task.Signature{ID: "publication-attempt", GroupID: "group", Name: "task"}
	if err := attemptBackend.SetStatePendingAttempt(signature, "attempt-a"); err != nil {
		t.Fatal(err)
	}
	if changed, err := attemptBackend.FailPendingAttempt(signature, "attempt-a", "publish failed"); err != nil || !changed {
		t.Fatalf("matching compensation = (%t, %v), want true, nil", changed, err)
	}
	status, err := b.GetStatus(signature.ID)
	if err != nil || status.Status != task.StateFailure || status.Error != "publish failed" {
		t.Fatalf("matching status = %#v, %v", status, err)
	}
	createAt := status.CreateAt

	if err := attemptBackend.SetStatePendingAttempt(signature, "attempt-b"); err != nil {
		t.Fatal(err)
	}
	if changed, err := attemptBackend.FailPendingAttempt(signature, "attempt-a", "stale attempt"); err != nil || changed {
		t.Fatalf("stale compensation = (%t, %v), want false, nil", changed, err)
	}
	status, err = b.GetStatus(signature.ID)
	if err != nil || status.Status != task.StatePending || status.Error != "" {
		t.Fatalf("stale-attempt status = %#v, %v", status, err)
	}
	if !status.CreateAt.Equal(createAt) {
		t.Fatalf("create_at changed from %v to %v on a new attempt", createAt, status.CreateAt)
	}

	tests := []struct {
		name       string
		want       task.State
		transition func(*task.Signature) error
	}{
		{name: "received", want: task.StateReceived, transition: b.SetStateReceived},
		{name: "started", want: task.StateStarted, transition: b.SetStateStarted},
		{name: "retry", want: task.StateRetry, transition: b.SetStateRetry},
		{name: "success", want: task.StateSuccess, transition: func(signature *task.Signature) error { return b.SetStateSuccess(signature, nil) }},
		{name: "failure", want: task.StateFailure, transition: func(signature *task.Signature) error { return b.SetStateFailure(signature, "worker failed") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig := &task.Signature{ID: "advanced-" + tt.name, GroupID: "group", Name: "task"}
			if err := attemptBackend.SetStatePendingAttempt(sig, "attempt"); err != nil {
				t.Fatal(err)
			}
			if err := tt.transition(sig); err != nil {
				t.Fatal(err)
			}
			if changed, err := attemptBackend.FailPendingAttempt(sig, "attempt", "publish failed"); err != nil || changed {
				t.Fatalf("advanced compensation = (%t, %v), want false, nil", changed, err)
			}
			status, err := b.GetStatus(sig.ID)
			if err != nil || status.Status != tt.want {
				t.Fatalf("advanced status = %#v, %v; want %s", status, err, tt.want)
			}
		})
	}
}

func TestPublicationAttemptCompensationLegacyRecordDoesNotMatch(t *testing.T) {
	b := newSQLiteBackend(t)
	attemptBackend, ok := interface{}(b).(publicationAttemptBackendForTest)
	if !ok {
		t.Fatal("BackendSQLDB does not implement atomic publication-attempt compensation")
	}
	signature := &task.Signature{ID: "legacy-pending", GroupID: "group", Name: "task"}
	if err := attemptBackend.SetStatePendingAttempt(signature, "stale-owner"); err != nil {
		t.Fatal(err)
	}
	if err := b.SetStatePending(signature); err != nil {
		t.Fatal(err)
	}
	if changed, err := attemptBackend.FailPendingAttempt(signature, "stale-owner", "publish failed"); err != nil || changed {
		t.Fatalf("legacy compensation = (%t, %v), want false, nil", changed, err)
	}
	status, err := b.GetStatus(signature.ID)
	if err != nil || status.Status != task.StatePending {
		t.Fatalf("legacy status = %#v, %v", status, err)
	}
}

func TestPublicationAttemptColumnMigratesExistingStatusTable(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := gdb.AutoMigrate(&task.Status{}); err != nil {
		t.Fatalf("create legacy status table: %v", err)
	}
	legacy := &task.Status{TaskID: "legacy-before-migration", GroupID: "group", Name: "task", Status: task.StatePending}
	if err := gdb.Create(legacy).Error; err != nil {
		t.Fatalf("insert legacy status: %v", err)
	}

	b := &BackendSQLDB{gClient: gdb}
	if err := b.autoMigrate(); err != nil {
		t.Fatalf("migrate publication attempt column: %v", err)
	}
	if !gdb.Migrator().HasColumn(&task.Status{}, publicationAttemptColumn) {
		t.Fatalf("status table is missing %s after migration", publicationAttemptColumn)
	}
	if changed, err := b.FailPendingAttempt(
		&task.Signature{ID: legacy.TaskID},
		"unknown-attempt",
		"publish failed",
	); err != nil || changed {
		t.Fatalf("legacy row compensation = (%t, %v), want false, nil", changed, err)
	}
	if err := b.SetStatePendingAttempt(&task.Signature{ID: legacy.TaskID, GroupID: "group", Name: "task"}, "attempt"); err != nil {
		t.Fatalf("write publication attempt after migration: %v", err)
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
