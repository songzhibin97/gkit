package distributed

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/robfig/cron/v3"
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
		name         string
		value        *task.GroupCallback
		wantContext  string
		wantWorkflow bool
	}{
		{name: "nil group callback", wantContext: "group callback", wantWorkflow: true},
		{name: "nil group", value: &task.GroupCallback{Callback: validCallback()}, wantContext: "group", wantWorkflow: true},
		{name: "nil callback", value: &task.GroupCallback{Group: validGroup(validMember("m1"))}, wantContext: "callback", wantWorkflow: true},
		{name: "empty group", value: &task.GroupCallback{Group: validGroup(), Callback: validCallback()}, wantContext: "group", wantWorkflow: true},
		{name: "nil member", value: &task.GroupCallback{Group: validGroup(nil), Callback: validCallback()}, wantContext: "member", wantWorkflow: true},
		{name: "empty group id", value: &task.GroupCallback{Group: &task.Group{Tasks: []*task.Signature{validMember("m1")}}, Callback: validCallback()}, wantContext: "group id"},
		{name: "empty callback id", value: &task.GroupCallback{Group: validGroup(validMember("m1")), Callback: &task.Signature{Name: "callback"}}, wantContext: "callback id"},
		{name: "empty member id", value: &task.GroupCallback{Group: validGroup(validMember("")), Callback: validCallback()}, wantContext: "member id"},
		{name: "duplicate member id", value: &task.GroupCallback{Group: validGroup(validMember("m1"), validMember("m1")), Callback: validCallback()}, wantContext: "duplicate member id"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &issue104ValidationBackend{}
			controller := &groupTestController{}
			server := &Server{
				config:     &Config{EnableDurableChordRegistration: true},
				backend:    backend,
				controller: controller,
			}

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
			if tt.wantWorkflow && !errors.Is(validationErr, task.ErrInvalidWorkflow) {
				t.Fatalf("validation error = %v, want ErrInvalidWorkflow joined with ErrChordInvalidInput", validationErr)
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

func TestDurableTimedChordValidationHasTypedFieldErrorsAndZeroSideEffects(t *testing.T) {
	validMember := func(id string) *task.Signature {
		return &task.Signature{ID: id, Name: "member"}
	}
	validCallback := func() *task.Signature {
		return &task.Signature{ID: "callback", Name: "callback"}
	}

	tests := []struct {
		name         string
		groupID      string
		callback     *task.Signature
		members      []*task.Signature
		wantContext  string
		wantWorkflow bool
	}{
		{name: "empty group id", callback: validCallback(), members: []*task.Signature{validMember("m1")}, wantContext: "group id"},
		{name: "empty callback id", groupID: "group", callback: &task.Signature{Name: "callback"}, members: []*task.Signature{validMember("m1")}, wantContext: "callback id"},
		{name: "empty member id", groupID: "group", callback: validCallback(), members: []*task.Signature{validMember("")}, wantContext: "member id"},
		{name: "duplicate member id", groupID: "group", callback: validCallback(), members: []*task.Signature{validMember("m1"), validMember("m1")}, wantContext: "duplicate member id"},
		{name: "empty group", groupID: "group", callback: validCallback(), wantContext: "empty group", wantWorkflow: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &issue104ValidationBackend{}
			controller := &groupTestController{}
			server := &Server{
				config:     &Config{EnableDurableChordRegistration: true},
				backend:    backend,
				controller: controller,
				scheduler:  cron.New(),
			}

			var validationErr error
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("RegisteredTimedGroupCallback panicked: %v", recovered)
					}
				}()
				validationErr = server.RegisteredTimedGroupCallback(
					"* * * * *",
					"timed-chord",
					tt.groupID,
					1,
					tt.callback,
					tt.members...,
				)
			}()
			if !errors.Is(validationErr, backendapi.ErrChordInvalidInput) {
				t.Fatalf("validation error = %v, want ErrChordInvalidInput", validationErr)
			}
			if tt.wantWorkflow && !errors.Is(validationErr, task.ErrInvalidWorkflow) {
				t.Fatalf("validation error = %v, want ErrInvalidWorkflow joined with ErrChordInvalidInput", validationErr)
			}
			if !strings.Contains(validationErr.Error(), tt.wantContext) {
				t.Fatalf("validation error = %q, want %q field context", validationErr, tt.wantContext)
			}

			if backend.takeovers != 0 || len(backend.pendingIDs) != 0 || controller.publishCount.Load() != 0 || len(server.scheduler.Entries()) != 0 {
				t.Fatalf(
					"invalid input caused side effects: takeovers=%d pending=%v publishes=%d scheduled=%d",
					backend.takeovers,
					backend.pendingIDs,
					controller.publishCount.Load(),
					len(server.scheduler.Entries()),
				)
			}
		})
	}
}
