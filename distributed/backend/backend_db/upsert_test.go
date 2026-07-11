package backend_db

import (
	"testing"

	"github.com/glebarez/sqlite"
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
