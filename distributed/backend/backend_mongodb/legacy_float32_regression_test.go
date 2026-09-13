package backend_mongodb

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/backend/result"
	"github.com/songzhibin97/gkit/distributed/task"
	"go.mongodb.org/mongo-driver/bson"
)

type legacyFloatResult struct {
	Type  string      `json:"type" bson:"type"`
	Value interface{} `json:"value" bson:"value"`
}

func TestIssue167MongoLegacyFloat32(t *testing.T) {
	for _, original := range []interface{}{float32(0.1), float32(math.MaxFloat32), []float32{0.1, 0.2, math.MaxFloat32}, float64(0.1)} {
		t.Run(fmt.Sprintf("%T-%v", original, original), func(t *testing.T) {
			b := issue167Mongo(t)
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			kind := reflect.TypeOf(original).String()
			// Reproduce the old JSON clone before writing the result into real BSON.
			encoded, err := json.Marshal(legacyFloatResult{Type: kind, Value: original})
			if err != nil {
				t.Fatal(err)
			}
			var legacy legacyFloatResult
			if err := json.Unmarshal(encoded, &legacy); err != nil {
				t.Fatal(err)
			}
			if _, err := b.taskTable.InsertOne(ctx, bson.M{"_id": "legacy-status", "status": task.StateSuccess, "results": []legacyFloatResult{legacy}}); err != nil {
				t.Fatal(err)
			}
			values, err := result.NewAsyncResult(&task.Signature{ID: "legacy-status"}, b).GetWithTimeout(time.Second, time.Millisecond)
			if err != nil {
				t.Error(err)
			} else if len(values) != 1 || !reflect.DeepEqual(values[0].Interface(), original) {
				t.Errorf("legacy status consumer=%v want %T(%v)", values, original, original)
			}
			callback, err := json.Marshal(&task.Signature{ID: "callback", Name: "float-consumer"})
			if err != nil {
				t.Fatal(err)
			}
			reg := backend.ChordRegistration{GroupID: "legacy-float-group", Callback: callback, Retention: -1}
			for i := 0; i < 2; i++ {
				id := fmt.Sprintf("member-%d", i)
				payload, err := json.Marshal(&task.Signature{ID: id, GroupID: reg.GroupID})
				if err != nil {
					t.Fatal(err)
				}
				reg.Members = append(reg.Members, backend.ChordMemberRegistration{Ordinal: i, TaskID: id, Payload: payload})
			}
			ref, err := b.RegisterChord(ctx, reg)
			if err != nil {
				t.Fatal(err)
			}
			if err := b.ReconcileChord(ctx, ref.DeliveryKey); err != nil {
				t.Fatal(err)
			}
			receipt := bson.M{"task_id": reg.Members[0].TaskID, "outcome": backend.MemberTerminalSuccess, "results": []legacyFloatResult{legacy}}
			if _, err := b.chordTable.UpdateOne(ctx, bson.M{"_id": ref.DeliveryKey}, bson.M{"$set": bson.M{"members.0.receipt": receipt, "members.0.state": backend.ChordMemberTerminal}}); err != nil {
				t.Fatal(err)
			}
			if err := b.RecordMemberTerminal(ctx, ref.DeliveryKey, 1, reg.Members[1].TaskID, backend.MemberTerminalSuccess, nil); err != nil {
				t.Fatal(err)
			}
			lease, claimed, err := b.ClaimCallbackPublication(ctx, backend.ChordCallbackClaim{DeliveryKey: ref.DeliveryKey, Owner: "legacy-float-check", Now: time.Now()})
			if err != nil || !claimed {
				t.Fatalf("claim=%t %v", claimed, err)
			}
			var signature task.Signature
			if err := json.Unmarshal(lease.Payload, &signature); err != nil {
				t.Fatal(err)
			}
			if len(signature.Args) != 1 {
				t.Fatalf("callback args=%d", len(signature.Args))
			}
			value, err := task.ReflectValue(signature.Args[0].Type, signature.Args[0].Value)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(value.Interface(), original) {
				t.Fatalf("legacy callback=%v want %v", value.Interface(), original)
			}
		})
	}
}
