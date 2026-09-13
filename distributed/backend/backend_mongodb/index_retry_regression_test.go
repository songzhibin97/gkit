package backend_mongodb

import (
	"context"
	"errors"
	"github.com/songzhibin97/gkit/distributed/backend"
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
