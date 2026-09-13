package backend_mongodb

import (
	"context"
	"errors"
	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
	"testing"
)

func TestIssue167MongoStatusContext(t *testing.T) {
	b := issue167Mongo(t)
	sig := &task.Signature{ID: "context-status"}
	if err := b.SetStateSuccess(sig, []*task.Result{{Type: "int64", Value: int64(7)}}); err != nil {
		t.Fatal(err)
	}
	reader, ok := interface{}(b).(backend.ContextStatusBackend)
	if !ok {
		t.Fatal("Mongo status does not expose context")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := reader.GetStatusContext(ctx, sig.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled status: %v", err)
	}
	status, err := reader.GetStatusContext(context.Background(), sig.ID)
	if err != nil || len(status.Results) != 1 || status.Results[0].Value != int64(7) {
		t.Fatalf("healthy status after cancellation: %v %v", status, err)
	}
}
