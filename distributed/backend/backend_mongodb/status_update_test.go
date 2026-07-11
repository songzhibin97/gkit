package backend_mongodb

import (
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"go.mongodb.org/mongo-driver/bson"
)

func TestBuildTaskStatusUpdatePreservesCreateAtOnExistingRows(t *testing.T) {
	createAt := time.Unix(1_700_000_000, 0)
	fields := bson.M{
		"status": task.StatePending,
		"error":  "",
	}
	update := buildTaskStatusUpdate(fields, createAt)

	set, ok := update["$set"].(bson.M)
	if !ok {
		t.Fatalf("$set = %#v, want bson.M", update["$set"])
	}
	if _, exists := set["create_at"]; exists {
		t.Fatal("create_at must not be overwritten on an existing task")
	}
	if set["error"] != "" {
		t.Fatalf("error = %#v, want explicit empty string", set["error"])
	}
	setOnInsert, ok := update["$setOnInsert"].(bson.M)
	if !ok {
		t.Fatalf("$setOnInsert = %#v, want bson.M", update["$setOnInsert"])
	}
	if got := setOnInsert["create_at"]; got != createAt {
		t.Fatalf("insert create_at = %#v, want %v", got, createAt)
	}
}
