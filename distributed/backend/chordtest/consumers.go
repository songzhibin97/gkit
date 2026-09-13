package chordtest

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/backend/result"
	"github.com/songzhibin97/gkit/distributed/task"
)

// RunResultConsumers checks concrete values at the result and callback consumers,
// including BSON's unsigned-integer and binary representations.
func RunResultConsumers(t *testing.T, b backend.DurableChordBackend) {
	t.Helper()
	ordinary, ok := b.(backend.Backend)
	if !ok {
		t.Fatal("missing ordinary backend")
	}
	inputs := []interface{}{
		true, int(19), int8(-8), int16(-1600), int32(-32000), int64(9007199254740993),
		uint(19), uint8(255), uint16(1600), uint32(32000), uint64(9007199254740993), float32(1.25), float64(2.5), "result",
		[]bool{true, false}, []int{19, -19}, []int8{-8, 8}, []int16{-1600, 1600}, []int32{-32000, 32000}, []int64{9007199254740993, -9007199254740993},
		[]uint{19, 20}, []uint8{0, 128, 255}, []uint16{1600, 1601}, []uint32{32000, 32001}, []uint64{9007199254740993, 9007199254740994}, []float32{1.25, 2.5}, []float64{2.5, 5}, []string{"first", "second"},
	}
	results := make([]*task.Result, len(inputs))
	prefix := fmt.Sprintf("issue167-consumers-%d", time.Now().UnixNano())
	for i, input := range inputs {
		results[i] = &task.Result{Type: reflect.TypeOf(input).String(), Value: input}
		t.Run("async-"+results[i].Type, func(t *testing.T) {
			sig := &task.Signature{ID: fmt.Sprintf("%s-status-%d", prefix, i)}
			if err := ordinary.SetStateSuccess(sig, []*task.Result{results[i]}); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := ordinary.ResetTask(sig.ID); err != nil {
					t.Error(err)
				}
			})
			values, err := result.NewAsyncResult(sig, ordinary).GetWithTimeout(time.Second, time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			if len(values) != 1 || !reflect.DeepEqual(values[0].Interface(), input) {
				t.Fatalf("consumer values=%v want %T(%v)", values, input, input)
			}
		})
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	callback, err := json.Marshal(&task.Signature{ID: prefix + "-callback", Name: "consume-types"})
	if err != nil {
		t.Fatal(err)
	}
	reg := backend.ChordRegistration{GroupID: prefix, GroupName: "types", Retention: 1, Callback: callback}
	for i := 0; i < 2; i++ {
		id := fmt.Sprintf("%s-member-%d", prefix, i)
		payload, err := json.Marshal(&task.Signature{ID: id, GroupID: prefix, Name: "member"})
		if err != nil {
			t.Fatal(err)
		}
		reg.Members = append(reg.Members, backend.ChordMemberRegistration{TaskID: id, Ordinal: i, Payload: payload})
	}
	ref, err := b.RegisterChord(ctx, reg)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.ReconcileChord(ctx, ref.DeliveryKey); err != nil {
		t.Fatal(err)
	}
	// The bytes and unsigned slices are in the first receipt, so another
	// receipt forces a BSON reload before the callback payload is built.
	split := len(results) - 1
	if err := b.RecordMemberTerminal(ctx, ref.DeliveryKey, 0, reg.Members[0].TaskID, backend.MemberTerminalSuccess, results[:split]); err != nil {
		t.Fatal(err)
	}
	if err := b.RecordMemberTerminal(ctx, ref.DeliveryKey, 1, reg.Members[1].TaskID, backend.MemberTerminalSuccess, results[split:]); err != nil {
		t.Fatal(err)
	}
	if err := b.RecordMemberTerminal(ctx, ref.DeliveryKey, 0, reg.Members[0].TaskID, backend.MemberTerminalSuccess, results[:split]); err != nil {
		t.Error(err)
	}
	lease, claimed, err := b.ClaimCallbackPublication(ctx, backend.ChordCallbackClaim{DeliveryKey: ref.DeliveryKey, Owner: "consumer-check", Now: time.Now()})
	if err != nil || !claimed {
		t.Fatalf("callback claim %t %v", claimed, err)
	}
	var signature task.Signature
	if err := json.Unmarshal(lease.Payload, &signature); err != nil {
		t.Fatal(err)
	}
	if len(signature.Args) != len(inputs) {
		t.Fatalf("callback args=%d", len(signature.Args))
	}
	for i, arg := range signature.Args {
		t.Run("callback-"+arg.Type, func(t *testing.T) {
			value, err := task.ReflectValue(arg.Type, arg.Value)
			if err != nil {
				t.Fatal(err)
			}
			// Invoke a typed function using the same reflected arguments as workers.
			callback := reflect.MakeFunc(reflect.FuncOf([]reflect.Type{reflect.TypeOf(inputs[i])}, nil, false), func(args []reflect.Value) []reflect.Value {
				if !reflect.DeepEqual(args[0].Interface(), inputs[i]) {
					t.Errorf("callback argument=%v want %v", args[0].Interface(), inputs[i])
				}
				return nil
			})
			callback.Call([]reflect.Value{value})
		})
	}
	if err := b.RecordCallbackTerminal(ctx, ref.DeliveryKey, backend.CallbackTerminalSuccess); err != nil {
		t.Fatal(err)
	}
	if _, err := b.CleanupTerminalChordDeliveries(ctx, time.Now().Add(2*time.Second), 100); err != nil {
		t.Fatal(err)
	}
	if err := ordinary.ResetGroup(prefix); err != nil {
		t.Fatal(err)
	}
	for _, member := range reg.Members {
		if err := ordinary.ResetTask(member.TaskID); err != nil {
			t.Fatal(err)
		}
	}
	if err := ordinary.ResetTask(signature.ID); err != nil {
		t.Fatal(err)
	}
}
