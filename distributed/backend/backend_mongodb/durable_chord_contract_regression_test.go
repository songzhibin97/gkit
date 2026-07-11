package backend_mongodb

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/backend/chordtest"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestDurableChordContract(t *testing.T) {
	uri := os.Getenv("GKIT_MONGO_URI")
	if uri == "" {
		t.Skip("GKIT_MONGO_URI is required for live durable chord contract")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = client.Disconnect(context.Background()) })
	database := fmt.Sprintf("gkit_issue104_%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = client.Database(database).Drop(context.Background()) })
	value, err := NewBackendMongoDBE(client, -1, SetDatabaseName(database))
	if err != nil {
		t.Fatal(err)
	}
	chordtest.Run(t, value.(*BackendMongoDB))
}
