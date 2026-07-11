package backend_mongodb

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/event"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestCleanupTerminalChordDeliveriesCancellationAfterDeleteStarts(t *testing.T) {
	uri := os.Getenv("GKIT_MONGO_URI")
	if uri == "" {
		t.Skip("GKIT_MONGO_URI is required for live durable chord cleanup")
	}

	var armed atomic.Bool
	var deleteStarted atomic.Bool
	var cleanupCancel atomic.Value
	cleanupCancel.Store(context.CancelFunc(func() {}))
	monitor := &event.CommandMonitor{
		Started: func(_ context.Context, command *event.CommandStartedEvent) {
			if command.CommandName != "delete" || !armed.CompareAndSwap(true, false) {
				return
			}
			deleteStarted.Store(true)
			cleanupCancel.Load().(context.CancelFunc)()
		},
	}

	connectCtx, connectCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer connectCancel()
	client, err := mongo.Connect(connectCtx, options.Client().ApplyURI(uri).SetMonitor(monitor))
	if err != nil {
		t.Fatal(err)
	}
	database := fmt.Sprintf("gkit_issue106_cleanup_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Database(database).Drop(cleanupCtx); err != nil {
			t.Errorf("drop test database: %v", err)
		}
		if err := client.Disconnect(cleanupCtx); err != nil {
			t.Errorf("disconnect MongoDB client: %v", err)
		}
	})

	value, err := NewBackendMongoDBE(client, -1, SetDatabaseName(database))
	if err != nil {
		t.Fatal(err)
	}
	backend := value.(*BackendMongoDB)
	if _, err := backend.chordTable.InsertOne(context.Background(), bson.M{
		"_id":                "terminal-cleanup-cancellation",
		"terminal_expire_at": time.Now().Add(-time.Hour),
	}); err != nil {
		t.Fatalf("insert terminal chord delivery: %v", err)
	}

	cleanupCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cleanupCancel.Store(context.CancelFunc(cancel))
	armed.Store(true)
	deleted, err := backend.CleanupTerminalChordDeliveries(cleanupCtx, time.Now(), 1)
	if !deleteStarted.Load() {
		t.Fatal("DeleteMany command did not start before cleanup returned")
	}
	if deleted != 0 {
		t.Fatalf("deleted = %d, want 0 after canceled DeleteMany", deleted)
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("cleanup error = %v, want context.Canceled", err)
	}
	if !strings.Contains(err.Error(), "delete terminal chord deliveries") {
		t.Fatalf("cleanup error lacks operation context: %v", err)
	}
}
