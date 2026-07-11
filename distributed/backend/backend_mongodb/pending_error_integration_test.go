package backend_mongodb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"go.mongodb.org/mongo-driver/mongo"
	moption "go.mongodb.org/mongo-driver/mongo/options"
)

func TestNonFailureTransitionsClearPreviousError(t *testing.T) {
	uri := os.Getenv("GKIT_MONGODB_URI")
	if uri == "" {
		t.Skip("GKIT_MONGODB_URI is not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, moption.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	databaseName := fmt.Sprintf("gkit_pending_error_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := client.Database(databaseName).Drop(cleanupCtx); err != nil {
			t.Errorf("drop test database: %v", err)
		}
		if err := client.Disconnect(cleanupCtx); err != nil {
			t.Errorf("disconnect MongoDB client: %v", err)
		}
	})

	backend, err := NewBackendMongoDBE(
		client,
		-1,
		SetDatabaseName(databaseName),
		SetTableTaskName("tasks"),
		SetTableGroupName("groups"),
	)
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name       string
		wantState  task.State
		transition func(*task.Signature) error
	}{
		{name: "pending", wantState: task.StatePending, transition: backend.SetStatePending},
		{name: "received", wantState: task.StateReceived, transition: backend.SetStateReceived},
		{name: "started", wantState: task.StateStarted, transition: backend.SetStateStarted},
		{name: "retry", wantState: task.StateRetry, transition: backend.SetStateRetry},
		{name: "success", wantState: task.StateSuccess, transition: func(sig *task.Signature) error { return backend.SetStateSuccess(sig, nil) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature := &task.Signature{ID: "task-" + tt.name, GroupID: "group", Name: "task"}
			if err := backend.SetStateFailure(signature, "stale failure"); err != nil {
				t.Fatal(err)
			}
			failed, err := backend.GetStatus(signature.ID)
			if err != nil {
				t.Fatal(err)
			}
			if failed.Status != task.StateFailure || failed.Error != "stale failure" {
				t.Fatalf("failure state = (%s, %q), want (FAILURE, stale failure)", failed.Status, failed.Error)
			}
			if err := tt.transition(signature); err != nil {
				t.Fatal(err)
			}
			status, err := backend.GetStatus(signature.ID)
			if err != nil {
				t.Fatal(err)
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
