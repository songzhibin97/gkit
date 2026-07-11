package distributed

import (
	"context"
	"errors"
	"testing"

	"github.com/robfig/cron/v3"
	"github.com/songzhibin97/gkit/distributed/task"
)

type emptyWorkflowBackend struct {
	groupTestBackend
	takeovers int
}

func (b *emptyWorkflowBackend) GroupTakeOver(string, string, ...string) error {
	b.takeovers++
	return nil
}

func TestDirectSendRejectsInvalidWorkflowWithoutSideEffects(t *testing.T) {
	tests := []struct {
		name string
		send func(*Server) error
	}{
		{name: "nil chain", send: func(server *Server) error {
			_, err := server.SendChain(nil)
			return err
		}},
		{name: "empty chain", send: func(server *Server) error {
			_, err := server.SendChain(&task.Chain{})
			return err
		}},
		{name: "nil chain member", send: func(server *Server) error {
			_, err := server.SendChain(&task.Chain{Tasks: []*task.Signature{nil}})
			return err
		}},
		{name: "nil group", send: func(server *Server) error {
			_, err := server.SendGroupWithContext(context.Background(), nil, 1)
			return err
		}},
		{name: "empty group", send: func(server *Server) error {
			_, err := server.SendGroupWithContext(context.Background(), &task.Group{GroupID: "group-id"}, 1)
			return err
		}},
		{name: "nil group member", send: func(server *Server) error {
			_, err := server.SendGroupWithContext(context.Background(), &task.Group{GroupID: "group-id", Tasks: []*task.Signature{nil}}, 1)
			return err
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backend := &emptyWorkflowBackend{}
			controller := &groupTestController{}
			server := &Server{backend: backend, controller: controller}
			var err error
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("send panicked: %v", recovered)
					}
				}()
				err = tt.send(server)
			}()
			if !errors.Is(err, task.ErrInvalidWorkflow) {
				t.Fatalf("error = %v, want task.ErrInvalidWorkflow", err)
			}
			if backend.takeovers != 0 || len(backend.pendingIDs) != 0 || controller.publishCount.Load() != 0 {
				t.Fatalf("invalid workflow caused side effects: takeovers=%d pending=%v publishes=%d", backend.takeovers, backend.pendingIDs, controller.publishCount.Load())
			}
		})
	}
}

func TestTimedRegistrationRejectsEmptyWorkflowSynchronously(t *testing.T) {
	tests := []struct {
		name     string
		register func(*Server) error
	}{
		{name: "chain", register: func(server *Server) error {
			return server.RegisteredTimedChain("* * * * *", "chain")
		}},
		{name: "group", register: func(server *Server) error {
			return server.RegisteredTimedGroup("* * * * *", "group", "group-id", 1)
		}},
		{name: "group callback without members", register: func(server *Server) error {
			return server.RegisteredTimedGroupCallback("* * * * *", "chord", "group-id", 1, &task.Signature{ID: "callback"})
		}},
		{name: "group callback without callback", register: func(server *Server) error {
			return server.RegisteredTimedGroupCallback("* * * * *", "chord", "group-id", 1, nil, &task.Signature{ID: "task"})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{scheduler: cron.New()}
			err := tt.register(server)
			if !errors.Is(err, task.ErrInvalidWorkflow) {
				t.Fatalf("error = %v, want task.ErrInvalidWorkflow", err)
			}
			if got := len(server.scheduler.Entries()); got != 0 {
				t.Fatalf("scheduled entries = %d, want 0", got)
			}
		})
	}
}

func TestTimedRegistrationPreservesCronParseErrorPriority(t *testing.T) {
	const invalidSpec = "not a cron expression"
	_, wantErr := cron.ParseStandard(invalidSpec)
	if wantErr == nil {
		t.Fatal("invalid test cron unexpectedly parsed")
	}

	tests := []struct {
		name     string
		register func(*Server) error
	}{
		{name: "chain", register: func(server *Server) error {
			return server.RegisteredTimedChain(invalidSpec, "chain")
		}},
		{name: "group", register: func(server *Server) error {
			return server.RegisteredTimedGroup(invalidSpec, "group", "group-id", 1)
		}},
		{name: "group callback", register: func(server *Server) error {
			return server.RegisteredTimedGroupCallback(invalidSpec, "chord", "group-id", 1, &task.Signature{ID: "callback"})
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{scheduler: cron.New()}
			err := tt.register(server)
			if err == nil || err.Error() != wantErr.Error() {
				t.Fatalf("error = %v, want cron parse error %v", err, wantErr)
			}
			if errors.Is(err, task.ErrInvalidWorkflow) {
				t.Fatalf("error = %v, validation must not replace cron parse error", err)
			}
			if got := len(server.scheduler.Entries()); got != 0 {
				t.Fatalf("scheduled entries = %d, want 0", got)
			}
		})
	}
}

func TestTimedRegistrationPreservesNonEmptyWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		register func(*Server) error
	}{
		{name: "chain", register: func(server *Server) error {
			return server.RegisteredTimedChain("* * * * *", "chain", &task.Signature{ID: "task"})
		}},
		{name: "group", register: func(server *Server) error {
			return server.RegisteredTimedGroup("* * * * *", "group", "group-id", 1, &task.Signature{ID: "task"})
		}},
		{name: "group callback", register: func(server *Server) error {
			return server.RegisteredTimedGroupCallback(
				"* * * * *",
				"chord",
				"group-id",
				1,
				&task.Signature{ID: "callback"},
				&task.Signature{ID: "task"},
			)
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := &Server{scheduler: cron.New()}
			if err := tt.register(server); err != nil {
				t.Fatalf("registration returned error: %v", err)
			}
			if got := len(server.scheduler.Entries()); got != 1 {
				t.Fatalf("scheduled entries = %d, want 1", got)
			}
		})
	}
}
