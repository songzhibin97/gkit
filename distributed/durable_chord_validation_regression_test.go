package distributed

import (
	"context"
	"testing"

	"github.com/songzhibin97/gkit/distributed/task"
)

type issue104ValidationBackend struct {
	groupTestBackend
	takeovers int
}

func (b *issue104ValidationBackend) GroupTakeOver(string, string, ...string) error {
	b.takeovers++
	return nil
}

func TestDurableChordValidationHasZeroSideEffects(t *testing.T) {
	validMember := func(id string) *task.Signature {
		return &task.Signature{ID: id, Name: "member"}
	}
	validCallback := func() *task.Signature {
		return &task.Signature{ID: "callback", Name: "callback"}
	}
	validGroup := func(members ...*task.Signature) *task.Group {
		return &task.Group{GroupID: "group", Name: "group", Tasks: members}
	}

	tests := []struct {
		name  string
		value *task.GroupCallback
	}{
		{name: "nil group callback"},
		{name: "nil group", value: &task.GroupCallback{Callback: validCallback()}},
		{name: "nil callback", value: &task.GroupCallback{Group: validGroup(validMember("m1"))}},
		{name: "empty group", value: &task.GroupCallback{Group: validGroup(), Callback: validCallback()}},
		{name: "nil member", value: &task.GroupCallback{Group: validGroup(nil), Callback: validCallback()}},
		{name: "empty group id", value: &task.GroupCallback{Group: &task.Group{Tasks: []*task.Signature{validMember("m1")}}, Callback: validCallback()}},
		{name: "empty callback id", value: &task.GroupCallback{Group: validGroup(validMember("m1")), Callback: &task.Signature{Name: "callback"}}},
		{name: "empty member id", value: &task.GroupCallback{Group: validGroup(validMember("")), Callback: validCallback()}},
		{name: "duplicate member id", value: &task.GroupCallback{Group: validGroup(validMember("m1"), validMember("m1")), Callback: validCallback()}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &issue104ValidationBackend{}
			controller := &groupTestController{}
			server := &Server{backend: backend, controller: controller}

			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("SendGroupCallbackWithContext panicked: %v", recovered)
					}
				}()
				if _, err := server.SendGroupCallbackWithContext(context.Background(), tt.value, 1); err == nil {
					t.Fatal("SendGroupCallbackWithContext returned nil error")
				}
			}()

			if backend.takeovers != 0 || len(backend.pendingIDs) != 0 || controller.publishCount.Load() != 0 {
				t.Fatalf("invalid input caused side effects: takeovers=%d pending=%v publishes=%d", backend.takeovers, backend.pendingIDs, controller.publishCount.Load())
			}
		})
	}
}
