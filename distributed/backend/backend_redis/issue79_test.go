package backend_redis

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestGroupTakeOverDuplicateReturnsConflictPromptly(t *testing.T) {
	backend, _ := newMockBackend(t, -1)
	const groupID = "issue-79-duplicate-group"
	if err := backend.GroupTakeOver(groupID, "group", "task-0"); err != nil {
		t.Fatalf("first GroupTakeOver returned error: %v", err)
	}

	done := make(chan error, 1)
	go func() {
		done <- backend.GroupTakeOver(groupID, "group", "task-0")
	}()

	select {
	case err := <-done:
		if !errors.Is(err, ErrGroupAlreadyExists) {
			t.Fatalf("duplicate GroupTakeOver error = %v, want ErrGroupAlreadyExists", err)
		}
		if !strings.Contains(err.Error(), groupID) {
			t.Fatalf("duplicate GroupTakeOver error = %q, want group ID context", err)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("duplicate GroupTakeOver did not return promptly")
	}
}
