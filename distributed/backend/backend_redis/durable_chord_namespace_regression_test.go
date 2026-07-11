package backend_redis

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
)

func TestRedisRawKeyWritersRejectDurableChordNamespace(t *testing.T) {
	tests := []struct {
		name  string
		write func(*BackendRedis, string) error
	}{
		{name: "group takeover", write: func(b *BackendRedis, id string) error { return b.GroupTakeOver(id, "group", "member") }},
		{name: "group completion", write: func(b *BackendRedis, id string) error { _, err := b.TriggerCompleted(id); return err }},
		{name: "task pending", write: func(b *BackendRedis, id string) error {
			return b.SetStatePending(&task.Signature{ID: id, Name: "task"})
		}},
		{name: "task received", write: func(b *BackendRedis, id string) error {
			return b.SetStateReceived(&task.Signature{ID: id, Name: "task"})
		}},
		{name: "task started", write: func(b *BackendRedis, id string) error {
			return b.SetStateStarted(&task.Signature{ID: id, Name: "task"})
		}},
		{name: "task retry", write: func(b *BackendRedis, id string) error { return b.SetStateRetry(&task.Signature{ID: id, Name: "task"}) }},
		{name: "task success", write: func(b *BackendRedis, id string) error {
			return b.SetStateSuccess(&task.Signature{ID: id, Name: "task"}, nil)
		}},
		{name: "task failure", write: func(b *BackendRedis, id string) error {
			return b.SetStateFailure(&task.Signature{ID: id, Name: "task"}, "failure")
		}},
		{name: "task reset", write: func(b *BackendRedis, id string) error { return b.ResetTask(id) }},
		{name: "group reset", write: func(b *BackendRedis, id string) error { return b.ResetGroup(id) }},
	}

	for index, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			b := NewBackendRedis(client, -1).(*BackendRedis)
			id := fmt.Sprintf("gkit:chord:{user-%d}:record", index)
			const legacyValue = "legacy-user-value"
			if err := client.Set(context.Background(), id, legacyValue, 0).Err(); err != nil {
				t.Fatalf("seed legacy key: %v", err)
			}

			err := tt.write(b, id)
			if !errors.Is(err, backend.ErrChordInvalidInput) {
				t.Fatalf("write error = %v, want ErrChordInvalidInput for reserved Redis key", err)
			}
			if got := client.Get(context.Background(), id).Val(); got != legacyValue {
				t.Fatalf("reserved writer changed legacy value to %q", got)
			}
		})
	}

	t.Run("fresh reserved group", func(t *testing.T) {
		mr := miniredis.RunT(t)
		client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
		t.Cleanup(func() { _ = client.Close() })
		b := NewBackendRedis(client, -1).(*BackendRedis)
		const id = "gkit:chord:{fresh-user}:record"
		if err := b.GroupTakeOver(id, "group", "member"); !errors.Is(err, backend.ErrChordInvalidInput) {
			t.Fatalf("fresh reserved GroupTakeOver error = %v, want ErrChordInvalidInput", err)
		}
		if _, err := client.Get(context.Background(), id).Result(); !errors.Is(err, redis.Nil) {
			t.Fatalf("fresh reserved key was created: %v", err)
		}
	})
}

func TestRedisChordRegistrationFailsClosedOnLegacyRecordCollision(t *testing.T) {
	for _, tt := range []struct {
		name string
		body func(*testing.T, string) []byte
	}{
		{
			name: "legacy task status",
			body: func(t *testing.T, id string) []byte {
				t.Helper()
				body, err := json.Marshal(task.NewPendingState(&task.Signature{ID: id, Name: "legacy"}))
				if err != nil {
					t.Fatal(err)
				}
				return body
			},
		},
		{name: "malformed legacy value", body: func(*testing.T, string) []byte { return []byte("legacy-non-json-value") }},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mr := miniredis.RunT(t)
			client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
			t.Cleanup(func() { _ = client.Close() })
			b := NewBackendRedis(client, -1).(*BackendRedis)
			registration := namespaceRegistration(t, "legacy-collision-"+tt.name)
			recordKey, err := redisChordRecordKey(registration.DeliveryKey)
			if err != nil {
				t.Fatal(err)
			}
			legacyBody := tt.body(t, recordKey)
			if err := client.Set(context.Background(), recordKey, legacyBody, 0).Err(); err != nil {
				t.Fatal(err)
			}

			if _, err := b.RegisterChord(context.Background(), registration); !errors.Is(err, backend.ErrChordRegistrationConflict) {
				t.Fatalf("RegisterChord error = %v, want ErrChordRegistrationConflict", err)
			}
			if got := client.Get(context.Background(), recordKey).Val(); got != string(legacyBody) {
				t.Fatalf("legacy record changed to %q", got)
			}
			if _, err := client.ZScore(context.Background(), redisChordDeliveryIndexKey, registration.DeliveryKey).Result(); !errors.Is(err, redis.Nil) {
				t.Fatalf("failed registration left delivery index: %v", err)
			}
			if _, err := client.HGet(context.Background(), redisChordIndexStateKey, registration.DeliveryKey).Result(); !errors.Is(err, redis.Nil) {
				t.Fatalf("failed registration left index state: %v", err)
			}
			if _, err := client.ZScore(context.Background(), redisChordDeliveryIndexKey, "").Result(); !errors.Is(err, redis.Nil) {
				t.Fatalf("legacy value created an empty delivery index: %v", err)
			}
		})
	}
}

func namespaceRegistration(t *testing.T, suffix string) backend.ChordRegistration {
	t.Helper()
	groupID := "namespace-group-" + suffix
	callback := task.NewSignature("namespace-callback-"+suffix, "callback")
	callbackBody, err := json.Marshal(callback)
	if err != nil {
		t.Fatal(err)
	}
	member := task.NewSignature("namespace-member-"+suffix, "member")
	memberBody, err := json.Marshal(member)
	if err != nil {
		t.Fatal(err)
	}
	registration := backend.ChordRegistration{
		GroupID:   groupID,
		GroupName: "namespace",
		Retention: -1,
		Callback:  callbackBody,
		Members:   []backend.ChordMemberRegistration{{Ordinal: 0, TaskID: member.ID, Payload: memberBody}},
	}
	if err := backend.FinalizeChordRegistration(&registration); err != nil {
		t.Fatal(err)
	}
	return registration
}
