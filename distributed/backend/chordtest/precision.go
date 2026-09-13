package chordtest

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
	"testing"
	"time"
)

// RunResultPrecision exercises separate persisted receipts and legacy status reads.
func RunResultPrecision(t *testing.T, b backend.DurableChordBackend) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	groupID := fmt.Sprintf("issue167-precision-%d", time.Now().UnixNano())
	reg := backend.ChordRegistration{GroupID: groupID, GroupName: "precision", Retention: 1}
	callback, err := json.Marshal(&task.Signature{ID: groupID + "-callback", Name: "precision"})
	if err != nil {
		t.Fatal(err)
	}
	reg.Callback = callback
	inputs := []*task.Result{{Type: "int64", Value: int64(9007199254740993)}, {Type: "[]int64", Value: []int64{9007199254740993, -9007199254740993}}, {Type: "int64", Value: int64(9223372036854775807)}}
	wants := []string{"9007199254740993", "[9007199254740993,-9007199254740993]", "9223372036854775807"}
	for i := range inputs {
		id := fmt.Sprintf("%s-member-%d", groupID, i)
		payload, err := json.Marshal(&task.Signature{ID: id, Name: "member", GroupID: groupID})
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
	for i, input := range inputs {
		if err := b.RecordMemberTerminal(ctx, ref.DeliveryKey, i, reg.Members[i].TaskID, backend.MemberTerminalSuccess, []*task.Result{input}); err != nil {
			t.Fatal(err)
		}
	}
	// Replaying an earlier receipt after all intervening writes must remain equal.
	if err := b.RecordMemberTerminal(ctx, ref.DeliveryKey, 0, reg.Members[0].TaskID, backend.MemberTerminalSuccess, []*task.Result{inputs[0]}); err != nil {
		t.Error(err)
	}
	lease, claimed, err := b.ClaimCallbackPublication(ctx, backend.ChordCallbackClaim{DeliveryKey: ref.DeliveryKey, Owner: "precision-check", Now: time.Now()})
	if err != nil || !claimed {
		t.Fatalf("claim %t %v", claimed, err)
	}
	var decoded task.Signature
	if err := json.Unmarshal(lease.Payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded.Args) != len(wants) {
		t.Fatalf("callback args=%d", len(decoded.Args))
	}
	for i, arg := range decoded.Args {
		body, err := json.Marshal(arg.Value)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != wants[i] {
			t.Errorf("callback[%d]=%s want %s", i, body, wants[i])
		}
	}
	ordinary, ok := b.(backend.Backend)
	if !ok {
		t.Fatal("missing ordinary backend")
	}
	for i, input := range inputs {
		if err := ordinary.SetStateSuccess(&task.Signature{ID: reg.Members[i].TaskID, GroupID: groupID}, []*task.Result{input}); err != nil {
			t.Fatal(err)
		}
	}
	statuses, err := ordinary.GroupTaskStatus(groupID)
	if err != nil {
		t.Fatal(err)
	}
	if len(statuses) != len(wants) {
		t.Fatalf("group statuses=%d", len(statuses))
	}
	for i, status := range statuses {
		if status.TaskID != reg.Members[i].TaskID || len(status.Results) != 1 {
			t.Fatalf("group[%d]=%#v", i, status)
		}
		body, err := json.Marshal(status.Results[0].Value)
		if err != nil {
			t.Fatal(err)
		}
		if string(body) != wants[i] {
			t.Errorf("group[%d]=%s want %s", i, body, wants[i])
		}
	}
	if err := b.RecordCallbackTerminal(ctx, ref.DeliveryKey, backend.CallbackTerminalSuccess); err != nil {
		t.Fatal(err)
	}
	if _, err := b.CleanupTerminalChordDeliveries(ctx, time.Now().Add(2*time.Second), 100); err != nil {
		t.Fatal(err)
	}
	if err := ordinary.ResetGroup(groupID); err != nil {
		t.Fatal(err)
	}
	for _, member := range reg.Members {
		if err := ordinary.ResetTask(member.TaskID); err != nil {
			t.Fatal(err)
		}
	}
	if err := ordinary.ResetTask(decoded.ID); err != nil {
		t.Fatal(err)
	}
}
