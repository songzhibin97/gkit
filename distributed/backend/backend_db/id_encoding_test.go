package backend_db

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
)

const reservedStringSlicePrefix = "gkit:string-slice:v1:"

func TestSQLGroupCommaTaskIDsRoundTripWithoutFalseCompletion(t *testing.T) {
	b := newSQLiteBackend(t)
	groupID := "group,one"
	wantTaskIDs := []string{"task,one", "task-two"}
	if err := b.GroupTakeOver(groupID, "group", wantTaskIDs...); err != nil {
		t.Fatalf("GroupTakeOver: %v", err)
	}

	group, err := b.getGroup(groupID)
	if err != nil {
		t.Fatalf("getGroup: %v", err)
	}
	if !reflect.DeepEqual([]string(group.TaskIDs), wantTaskIDs) {
		t.Fatalf("task IDs = %#v, want %#v", group.TaskIDs, wantTaskIDs)
	}

	for _, id := range []string{"task", "one", "task-two"} {
		if err := b.SetStateSuccess(&task.Signature{ID: id, GroupID: groupID, Name: id}, nil); err != nil {
			t.Fatalf("SetStateSuccess(%q): %v", id, err)
		}
	}
	completed, err := b.GroupCompleted(groupID)
	if err != nil {
		t.Fatalf("GroupCompleted: %v", err)
	}
	if completed {
		t.Fatal("group completed from statuses belonging to comma-split task IDs")
	}

	if err := b.SetStateSuccess(&task.Signature{ID: "task,one", GroupID: groupID, Name: "task,one"}, nil); err != nil {
		t.Fatalf("SetStateSuccess(comma ID): %v", err)
	}
	completed, err = b.GroupCompleted(groupID)
	if err != nil {
		t.Fatalf("GroupCompleted after intended statuses: %v", err)
	}
	if !completed {
		t.Fatal("group did not complete after both intended task IDs completed")
	}
}

func TestSQLGroupReadsLegacyTaskIDsAndPreservesOrdinaryWrites(t *testing.T) {
	b := newSQLiteBackend(t)
	legacyGroupID := "legacy,group"
	if err := b.gClient.Exec(
		"INSERT INTO group_meta (id, name, task_ids, trigger_chord, lock, ttl, create_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		legacyGroupID, "legacy", "legacy-a,legacy-b", false, false, -1, time.Now(),
	).Error; err != nil {
		t.Fatalf("insert legacy group: %v", err)
	}
	group, err := b.getGroup(legacyGroupID)
	if err != nil {
		t.Fatalf("get legacy group: %v", err)
	}
	wantLegacy := []string{"legacy-a", "legacy-b"}
	if !reflect.DeepEqual([]string(group.TaskIDs), wantLegacy) {
		t.Fatalf("legacy task IDs = %#v, want %#v", group.TaskIDs, wantLegacy)
	}

	ordinaryGroupID := "ordinary,group"
	if err := b.GroupTakeOver(ordinaryGroupID, "ordinary", "task-a", "task-b"); err != nil {
		t.Fatalf("GroupTakeOver ordinary: %v", err)
	}
	var raw string
	if err := b.gClient.Model(&task.GroupMeta{}).
		Select("task_ids").
		Where("id = ?", ordinaryGroupID).
		Scan(&raw).Error; err != nil {
		t.Fatalf("read raw task_ids: %v", err)
	}
	if raw != "task-a,task-b" {
		t.Fatalf("ordinary task_ids bytes = %q, want legacy encoding %q", raw, "task-a,task-b")
	}
}

// TestSQLGroupHistoricalMarkerRowsFollowReservedNamespace documents the
// unavoidable compatibility boundary: a historical legacy value beginning
// with the reserved prefix cannot be distinguished from a versioned value in
// the same TEXT column. Upgrade tooling must migrate such rows before rollout.
func TestSQLGroupHistoricalMarkerRowsFollowReservedNamespace(t *testing.T) {
	b := newSQLiteBackend(t)
	insertRaw := func(groupID, taskIDs string) {
		t.Helper()
		if err := b.gClient.Exec(
			"INSERT INTO group_meta (id, name, task_ids, trigger_chord, lock, ttl, create_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
			groupID, "legacy", taskIDs, false, false, -1, time.Now(),
		).Error; err != nil {
			t.Fatalf("insert raw group %q: %v", groupID, err)
		}
	}

	insertRaw("historical-marker-invalid", reservedStringSlicePrefix+"legacy-task")
	if _, err := b.getGroup("historical-marker-invalid"); err == nil {
		t.Fatal("invalid historical marker row was silently accepted")
	} else if !strings.Contains(err.Error(), "invalid versioned payload") {
		t.Fatalf("invalid historical marker error = %v, want invalid versioned payload", err)
	}

	insertRaw("historical-marker-valid", reservedStringSlicePrefix+`["decoded-as-versioned"]`)
	group, err := b.getGroup("historical-marker-valid")
	if err != nil {
		t.Fatalf("get valid-looking historical marker row: %v", err)
	}
	want := []string{"decoded-as-versioned"}
	if !reflect.DeepEqual([]string(group.TaskIDs), want) {
		t.Fatalf("valid-looking historical marker row = %#v, want versioned interpretation %#v", group.TaskIDs, want)
	}
}
