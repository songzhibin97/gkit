package backend_mongodb

import (
	"errors"
	"math"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	moption "go.mongodb.org/mongo-driver/mongo/options"
)

func TestBuildTTLIndexModels(t *testing.T) {
	tests := []struct {
		name             string
		input            int64
		wantNormalized   int64
		wantModels       bool
		wantExpireSecond int32
	}{
		{
			name:             "zero uses default",
			input:            0,
			wantNormalized:   3600,
			wantModels:       true,
			wantExpireSecond: 3600,
		},
		{
			name:           "negative disables ttl",
			input:          -1,
			wantNormalized: -1,
			wantModels:     false,
		},
		{
			name:             "positive applies to task and group",
			input:            42,
			wantNormalized:   42,
			wantModels:       true,
			wantExpireSecond: 42,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalized, taskModels, groupModels, err := buildTTLIndexModels(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if normalized != tt.wantNormalized {
				t.Fatalf("normalized expiration = %d, want %d", normalized, tt.wantNormalized)
			}
			if !tt.wantModels {
				if len(taskModels) != 0 || len(groupModels) != 0 {
					t.Fatalf("negative expiration built TTL models: task=%d group=%d", len(taskModels), len(groupModels))
				}
				return
			}

			assertTTLIndexModel(t, taskModels, "gkit_tasks_create_at_ttl", tt.wantExpireSecond)
			assertTTLIndexModel(t, groupModels, "gkit_groups_create_at_ttl", tt.wantExpireSecond)
		})
	}
}

func TestBuildTTLIndexModelsRejectsInt32Overflow(t *testing.T) {
	if _, _, _, err := buildTTLIndexModels(int64(math.MaxInt32) + 1); err == nil {
		t.Fatal("expiration above int32 range did not return an error")
	}

	normalized, taskModels, groupModels, err := buildTTLIndexModels(math.MaxInt32)
	if err != nil {
		t.Fatalf("math.MaxInt32 should be accepted: %v", err)
	}
	if normalized != math.MaxInt32 {
		t.Fatalf("normalized expiration = %d, want %d", normalized, int64(math.MaxInt32))
	}
	assertTTLIndexModel(t, taskModels, "gkit_tasks_create_at_ttl", math.MaxInt32)
	assertTTLIndexModel(t, groupModels, "gkit_groups_create_at_ttl", math.MaxInt32)
}

func assertTTLIndexModel(t *testing.T, models []mongo.IndexModel, wantName string, wantExpireSeconds int32) {
	t.Helper()
	if len(models) != 1 {
		t.Fatalf("TTL model count = %d, want 1", len(models))
	}
	model := models[0]
	wantKeys := bson.D{{Key: "create_at", Value: 1}}
	if !reflect.DeepEqual(model.Keys, wantKeys) {
		t.Fatalf("TTL model keys = %#v, want %#v", model.Keys, wantKeys)
	}
	if model.Options == nil || model.Options.Name == nil || *model.Options.Name != wantName {
		t.Fatalf("TTL model name = %v, want %q", model.Options, wantName)
	}
	if model.Options.ExpireAfterSeconds == nil || *model.Options.ExpireAfterSeconds != wantExpireSeconds {
		t.Fatalf("TTL model expiration = %v, want %d", model.Options.ExpireAfterSeconds, wantExpireSeconds)
	}
}

func TestMongoConstructorsSurfaceIndexSetupFailure(t *testing.T) {
	client, err := mongo.NewClient(moption.Client().
		ApplyURI("mongodb://127.0.0.1:1").
		SetConnectTimeout(5 * time.Millisecond).
		SetServerSelectionTimeout(5 * time.Millisecond))
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	got, err := NewBackendMongoDBE(client, 1)
	if err == nil {
		t.Fatal("NewBackendMongoDBE returned nil error for a disconnected client")
	}
	if got != nil {
		t.Fatalf("NewBackendMongoDBE backend = %T, want nil on initialization failure", got)
	}
	if !errors.Is(err, mongo.ErrClientDisconnected) {
		t.Fatalf("NewBackendMongoDBE error = %v, want wrapped mongo.ErrClientDisconnected", err)
	}
	if !strings.Contains(err.Error(), "create task TTL index") {
		t.Fatalf("NewBackendMongoDBE error lacks operation context: %v", err)
	}
	if strings.Contains(err.Error(), "mongodb://") {
		t.Fatalf("NewBackendMongoDBE error exposed connection details: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("NewBackendMongoDBE took %s, want a prompt bounded failure", elapsed)
	}

	start = time.Now()
	if legacy := NewBackendMongoDB(client, 1); legacy != nil {
		t.Fatalf("NewBackendMongoDB backend = %T, want nil on initialization failure", legacy)
	}
	if elapsed := time.Since(start); elapsed > 250*time.Millisecond {
		t.Fatalf("NewBackendMongoDB took %s, want a prompt bounded failure", elapsed)
	}
}

func TestMongoNegativeExpirationSkipsIndexSetup(t *testing.T) {
	client, err := mongo.NewClient()
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewBackendMongoDBE(client, -1)
	if err != nil {
		t.Fatalf("negative expiration should not access MongoDB: %v", err)
	}
	if b == nil {
		t.Fatal("negative expiration returned a nil backend")
	}

	impl := b.(*BackendMongoDB)
	impl.SetResultExpire(0)
	if impl.resultExpire != 3600 {
		t.Fatalf("SetResultExpire(0) stored %d, want 3600", impl.resultExpire)
	}
	impl.SetResultExpire(-1)
	if impl.resultExpire != -1 {
		t.Fatalf("SetResultExpire(-1) stored %d, want -1", impl.resultExpire)
	}
}
