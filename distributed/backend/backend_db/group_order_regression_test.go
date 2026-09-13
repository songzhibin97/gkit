package backend_db

import (
	"github.com/songzhibin97/gkit/distributed/task"
	"testing"
)

func TestGroupTaskStatusPreservesMemberOrder(t *testing.T) {
	b := newSQLiteBackend(t)
	b.SetResultExpire(-1)
	for _, id := range []string{"a", "z"} {
		if err := b.SetStateSuccess(&task.Signature{ID: id}, []*task.Result{{Type: "int64", Value: int64(10)}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.GroupTakeOver("ordered-group", "subtract", "z", "missing", "a"); err != nil {
		t.Fatal(err)
	}
	statuses, err := b.GroupTaskStatus("ordered-group")
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != 2 || statuses[0].TaskID != "z" || statuses[1].TaskID != "a" {
		t.Fatalf("member order = %#v", statuses)
	}
	completed, err := b.GroupCompleted("ordered-group")
	if err != nil || completed {
		t.Fatalf("missing member: completed=%t, err=%v", completed, err)
	}
}
