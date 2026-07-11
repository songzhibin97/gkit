package backend_mongodb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	moption "go.mongodb.org/mongo-driver/mongo/options"
)

func newMongoIntegrationBackend(t *testing.T) *BackendMongoDB {
	t.Helper()
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

	constructed, err := NewBackendMongoDBE(
		client,
		-1,
		SetDatabaseName(databaseName),
		SetTableTaskName("tasks"),
		SetTableGroupName("groups"),
	)
	if err != nil {
		t.Fatal(err)
	}
	backend, ok := constructed.(*BackendMongoDB)
	if !ok {
		t.Fatalf("backend type = %T, want *BackendMongoDB", constructed)
	}
	return backend
}

func TestNonFailureTransitionsClearPreviousError(t *testing.T) {
	backend := newMongoIntegrationBackend(t)
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

func TestPublicationAttemptCompensation(t *testing.T) {
	backend := newMongoIntegrationBackend(t)
	signature := &task.Signature{ID: "publication-attempt", GroupID: "group", Name: "task"}

	if err := backend.SetStatePendingAttempt(signature, "attempt-a"); err != nil {
		t.Fatal(err)
	}
	if changed, err := backend.FailPendingAttempt(signature, "attempt-a", "publish failed"); err != nil || !changed {
		t.Fatalf("matching compensation = (%t, %v), want true, nil", changed, err)
	}
	status, err := backend.GetStatus(signature.ID)
	if err != nil || status.Status != task.StateFailure || status.Error != "publish failed" {
		t.Fatalf("matching status = %#v, %v", status, err)
	}
	createAt := status.CreateAt

	if err := backend.SetStatePendingAttempt(signature, "attempt-b"); err != nil {
		t.Fatal(err)
	}
	if changed, err := backend.FailPendingAttempt(signature, "attempt-a", "stale attempt"); err != nil || changed {
		t.Fatalf("stale compensation = (%t, %v), want false, nil", changed, err)
	}
	status, err = backend.GetStatus(signature.ID)
	if err != nil || status.Status != task.StatePending || status.Error != "" {
		t.Fatalf("stale-attempt status = %#v, %v", status, err)
	}
	if !status.CreateAt.Equal(createAt) {
		t.Fatalf("create_at changed from %v to %v on a new attempt", createAt, status.CreateAt)
	}
}

func TestPublicationAttemptCompensationPreservesAdvancedState(t *testing.T) {
	backend := newMongoIntegrationBackend(t)
	tests := []struct {
		name       string
		want       task.State
		transition func(*task.Signature) error
	}{
		{name: "received", want: task.StateReceived, transition: backend.SetStateReceived},
		{name: "started", want: task.StateStarted, transition: backend.SetStateStarted},
		{name: "retry", want: task.StateRetry, transition: backend.SetStateRetry},
		{name: "success", want: task.StateSuccess, transition: func(signature *task.Signature) error { return backend.SetStateSuccess(signature, nil) }},
		{name: "failure", want: task.StateFailure, transition: func(signature *task.Signature) error { return backend.SetStateFailure(signature, "worker failed") }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			signature := &task.Signature{ID: "advanced-" + tt.name, GroupID: "group", Name: "task"}
			if err := backend.SetStatePendingAttempt(signature, "attempt"); err != nil {
				t.Fatal(err)
			}
			if err := tt.transition(signature); err != nil {
				t.Fatal(err)
			}
			if changed, err := backend.FailPendingAttempt(signature, "attempt", "publish failed"); err != nil || changed {
				t.Fatalf("advanced compensation = (%t, %v), want false, nil", changed, err)
			}
			status, err := backend.GetStatus(signature.ID)
			if err != nil || status.Status != tt.want {
				t.Fatalf("advanced status = %#v, %v; want %s", status, err, tt.want)
			}
		})
	}
}

func TestPublicationAttemptCompensationLegacyRecordDoesNotMatch(t *testing.T) {
	backend := newMongoIntegrationBackend(t)
	signature := &task.Signature{ID: "legacy-pending", GroupID: "group", Name: "task"}
	if err := backend.SetStatePendingAttempt(signature, "stale-owner"); err != nil {
		t.Fatal(err)
	}
	if err := backend.SetStatePending(signature); err != nil {
		t.Fatal(err)
	}
	if changed, err := backend.FailPendingAttempt(signature, "stale-owner", "publish failed"); err != nil || changed {
		t.Fatalf("legacy compensation = (%t, %v), want false, nil", changed, err)
	}
	status, err := backend.GetStatus(signature.ID)
	if err != nil || status.Status != task.StatePending {
		t.Fatalf("legacy status = %#v, %v", status, err)
	}
	var stored bson.M
	if err := backend.taskTable.FindOne(context.Background(), bson.M{"_id": signature.ID}).Decode(&stored); err != nil {
		t.Fatal(err)
	}
	if got := stored[publicationAttemptField]; got != "" {
		t.Fatalf("legacy pending %s = %#v, want cleared ownership", publicationAttemptField, got)
	}

	missingSignature := &task.Signature{ID: "legacy-missing-metadata", GroupID: "group", Name: "task"}
	if _, err := backend.taskTable.InsertOne(context.Background(), bson.M{
		"_id":      missingSignature.ID,
		"status":   task.StatePending,
		"group_id": missingSignature.GroupID,
		"name":     missingSignature.Name,
	}); err != nil {
		t.Fatal(err)
	}
	if changed, err := backend.FailPendingAttempt(missingSignature, "unknown-attempt", "publish failed"); err != nil || changed {
		t.Fatalf("missing-metadata compensation = (%t, %v), want false, nil", changed, err)
	}
}
