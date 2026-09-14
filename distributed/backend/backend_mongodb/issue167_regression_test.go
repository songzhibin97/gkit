package backend_mongodb

import (
	"context"
	"fmt"
	"github.com/songzhibin97/gkit/distributed/task"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"os"
	"testing"
	"time"
)

func issue167Mongo(t *testing.T) *BackendMongoDB {
	t.Helper()
	uri := os.Getenv("GKIT_MONGO_URI")
	if uri == "" {
		t.Skip("#167 live regression requires GKIT_MONGO_URI; run in owned fixture before acceptance")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	db := fmt.Sprintf("gkit_issue167_%d", time.Now().UnixNano())
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := client.Database(db).Drop(ctx); err != nil {
			t.Error(err)
		}
		if err := client.Disconnect(ctx); err != nil {
			t.Error(err)
		}
	})
	value, err := NewBackendMongoDBE(client, -1, SetDatabaseName(db))
	if err != nil {
		t.Fatal(err)
	}
	return value.(*BackendMongoDB)
}

func TestIssue167MongoMemberOrder(t *testing.T) {
	b := issue167Mongo(t)
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
		t.Fatalf("unordered statuses: %#v", statuses)
	}
	if completed, err := b.GroupCompleted("ordered-group"); err != nil || completed {
		t.Fatalf("missing member: completed=%t err=%v", completed, err)
	}
}
