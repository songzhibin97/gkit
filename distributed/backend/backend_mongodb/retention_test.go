package backend_mongodb

import (
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"go.mongodb.org/mongo-driver/bson"
)

func TestBuildTaskStatusUpdateSetsCreateAtOnlyOnInsert(t *testing.T) {
	createdAt := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	fields := bson.M{"status": task.StateStarted}
	update := buildTaskStatusUpdate(fields, createdAt)

	setFields, ok := update["$set"].(bson.M)
	if !ok {
		t.Fatalf("$set = %#v, want bson.M", update["$set"])
	}
	if setFields["status"] != task.StateStarted {
		t.Fatalf("$set status = %#v, want %v", setFields["status"], task.StateStarted)
	}
	if _, ok := setFields["create_at"]; ok {
		t.Fatalf("$set unexpectedly refreshes create_at: %#v", setFields)
	}

	setOnInsert, ok := update["$setOnInsert"].(bson.M)
	if !ok {
		t.Fatalf("$setOnInsert = %#v, want bson.M", update["$setOnInsert"])
	}
	if got, ok := setOnInsert["create_at"].(time.Time); !ok || !got.Equal(createdAt) {
		t.Fatalf("$setOnInsert create_at = %#v, want %v", setOnInsert["create_at"], createdAt)
	}
}
