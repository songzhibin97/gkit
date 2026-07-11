package distributed

import (
	"context"
	"errors"
	"strings"
	"testing"

	backendapi "github.com/songzhibin97/gkit/distributed/backend"
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

func TestDurableChordValidationHasTypedFieldErrorsAndZeroSideEffects(t *testing.T) {
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
		name        string
		value       *task.GroupCallback
		wantContext string
	}{
		{name: "nil group callback", wantContext: "group callback"},
		{name: "nil group", value: &task.GroupCallback{Callback: validCallback()}, wantContext: "group"},
		{name: "nil callback", value: &task.GroupCallback{Group: validGroup(validMember("m1"))}, wantContext: "callback"},
		{name: "empty group", value: &task.GroupCallback{Group: validGroup(), Callback: validCallback()}, wantContext: "group"},
		{name: "nil member", value: &task.GroupCallback{Group: validGroup(nil), Callback: validCallback()}, wantContext: "member"},
		{name: "empty group id", value: &task.GroupCallback{Group: &task.Group{Tasks: []*task.Signature{validMember("m1")}}, Callback: validCallback()}, wantContext: "group id"},
		{name: "empty callback id", value: &task.GroupCallback{Group: validGroup(validMember("m1")), Callback: &task.Signature{Name: "callback"}}, wantContext: "callback id"},
		{name: "empty member id", value: &task.GroupCallback{Group: validGroup(validMember("")), Callback: validCallback()}, wantContext: "member id"},
		{name: "duplicate member id", value: &task.GroupCallback{Group: validGroup(validMember("m1"), validMember("m1")), Callback: validCallback()}, wantContext: "member id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &issue104ValidationBackend{}
			controller := &groupTestController{}
			server := &Server{backend: backend, controller: controller}

			var validationErr error
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("SendGroupCallbackWithContext panicked: %v", recovered)
					}
				}()
				_, validationErr = server.SendGroupCallbackWithContext(context.Background(), tt.value, 1)
			}()
			if !errors.Is(validationErr, backendapi.ErrChordInvalidInput) {
				t.Fatalf("validation error = %v, want ErrChordInvalidInput", validationErr)
			}
			if !strings.Contains(validationErr.Error(), tt.wantContext) {
				t.Fatalf("validation error = %q, want %q field context", validationErr, tt.wantContext)
			}

			if backend.takeovers != 0 || len(backend.pendingIDs) != 0 || controller.publishCount.Load() != 0 {
				t.Fatalf("invalid input caused side effects: takeovers=%d pending=%v publishes=%d", backend.takeovers, backend.pendingIDs, controller.publishCount.Load())
			}
		})
	}
}
