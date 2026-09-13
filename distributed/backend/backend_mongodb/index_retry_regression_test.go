package backend_mongodb

import (
	"context"
	"errors"
	"github.com/songzhibin97/gkit/distributed/backend"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"reflect"
	"testing"
	"time"
)

func TestIssue167MongoIndexRetry(t *testing.T) {
	b := issue167Mongo(t)
	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := b.ScanChordDeliveries(canceled, backend.ChordScan{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("first scan: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := b.ScanChordDeliveries(ctx, backend.ChordScan{}); err != nil {
		t.Fatalf("healthy retry: %v", err)
	}
	if _, err := b.ScanChordDeliveries(canceled, backend.ChordScan{}); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled initialized scan: %v", err)
	}
}

func TestIssue167MongoIndexCreationFailureRetry(t *testing.T) {
	b := issue167Mongo(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	const name = "gkit_chord_delivery_key"
	if _, err := b.chordTable.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: bson.D{{Key: "wrong_field", Value: 1}}, Options: options.Index().SetName(name)}); err != nil {
		t.Fatal(err)
	}
	_, err := b.ScanChordDeliveries(ctx, backend.ChordScan{})
	var commandErr mongo.CommandError
	if !errors.As(err, &commandErr) || (commandErr.Code != 85 && commandErr.Code != 86) {
		t.Fatalf("actual createIndexes conflict = %v", err)
	}
	if _, err := b.chordTable.Indexes().DropOne(ctx, name); err != nil {
		t.Fatal(err)
	}
	if _, err := b.ScanChordDeliveries(ctx, backend.ChordScan{}); err != nil {
		t.Fatalf("healthy retry after removing conflict: %v", err)
	}
	cursor, err := b.chordTable.Indexes().List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := cursor.Close(ctx); err != nil {
			t.Error(err)
		}
	}()
	var indexes []struct {
		Name   string `bson:"name"`
		Keys   bson.D `bson:"key"`
		Unique bool   `bson:"unique"`
	}
	if err := cursor.All(ctx, &indexes); err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		if index.Name == name {
			if !index.Unique || !reflect.DeepEqual(index.Keys, bson.D{{Key: "delivery_key", Value: int32(1)}}) {
				t.Fatalf("recovered index = %#v", index)
			}
			return
		}
	}
	t.Fatal("healthy retry did not create the delivery-key unique index")
}
