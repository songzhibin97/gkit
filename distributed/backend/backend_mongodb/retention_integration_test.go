package backend_mongodb

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	moption "go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoTaskRetentionMetadata(t *testing.T) {
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
	databaseName := fmt.Sprintf("gkit_retention_%d", time.Now().UnixNano())
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

	const expireSeconds int32 = 600
	backend, err := NewBackendMongoDBE(
		client,
		int64(expireSeconds),
		SetDatabaseName(databaseName),
		SetTableTaskName("tasks"),
		SetTableGroupName("groups"),
	)
	if err != nil {
		t.Fatal(err)
	}
	b := backend.(*BackendMongoDB)
	assertStoredTTLIndex(t, ctx, b.taskTable, taskTTLIndexName, expireSeconds)
	assertStoredTTLIndex(t, ctx, b.groupTable, groupTTLIndexName, expireSeconds)

	signature := &task.Signature{ID: "task-retention", GroupID: "group-retention", Name: "retention"}
	if err := b.SetStateStarted(signature); err != nil {
		t.Fatal(err)
	}
	status, err := b.GetStatus(signature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if status.CreateAt.IsZero() {
		t.Fatal("non-pending upsert stored a zero create_at, so the TTL index cannot expire it")
	}

	fixedCreateAt := time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC)
	if _, err := b.taskTable.UpdateOne(ctx, bson.M{"_id": signature.ID}, bson.M{"$set": bson.M{"create_at": fixedCreateAt}}); err != nil {
		t.Fatal(err)
	}
	if err := b.SetStatePending(signature); err != nil {
		t.Fatal(err)
	}
	status, err = b.GetStatus(signature.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !status.CreateAt.Equal(fixedCreateAt) {
		t.Fatalf("existing task create_at = %v, want original creation time %v", status.CreateAt, fixedCreateAt)
	}
}

func assertStoredTTLIndex(t *testing.T, ctx context.Context, collection *mongo.Collection, name string, expireSeconds int32) {
	t.Helper()
	specifications, err := collection.Indexes().ListSpecifications(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, specification := range specifications {
		if specification.Name != name {
			continue
		}
		var keys bson.D
		if err := bson.Unmarshal(specification.KeysDocument, &keys); err != nil {
			t.Fatal(err)
		}
		wantKeys := bson.D{{Key: "create_at", Value: int32(1)}}
		if !reflect.DeepEqual(keys, wantKeys) {
			t.Fatalf("TTL index %q keys = %#v, want %#v", name, keys, wantKeys)
		}
		if specification.ExpireAfterSeconds == nil || *specification.ExpireAfterSeconds != expireSeconds {
			t.Fatalf("TTL index %q expiration = %v, want %d", name, specification.ExpireAfterSeconds, expireSeconds)
		}
		return
	}
	t.Fatalf("TTL index %q was not created", name)
}
