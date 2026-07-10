package distributed

import (
	"context"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
)

type routingObservation struct {
	id     string
	router string
}

func TestSendTaskLateDefaultRouting(t *testing.T) {
	tests := []struct {
		name      string
		signature *task.Signature
		hook      func(*task.Signature)
		want      string
	}{
		{
			name:      "unset uses consume queue",
			signature: task.NewSignature("unset", "task"),
			want:      "custom-queue",
		},
		{
			name:      "pre-publish router is preserved",
			signature: task.NewSignature("hook", "task"),
			hook: func(signature *task.Signature) {
				signature.Router = "hook-queue"
			},
			want: "hook-queue",
		},
		{
			name:      "explicit router is preserved",
			signature: task.NewSignature("explicit", "task", task.SetRouter("explicit-queue")),
			want:      "explicit-queue",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			observed := make(chan routingObservation, 1)
			controller := &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
				observed <- routingObservation{id: signature.ID, router: signature.Router}
				return nil
			}}
			server := &Server{
				config:            &Config{ConsumeQueue: "custom-queue"},
				backend:           &groupTestBackend{},
				controller:        controller,
				prePublishHandler: test.hook,
			}

			if _, err := server.SendTaskWithContext(context.Background(), test.signature); err != nil {
				t.Fatalf("SendTaskWithContext returned error: %v", err)
			}
			observation := <-observed
			if observation.router != test.want {
				t.Fatalf("published router = %q, want %q", observation.router, test.want)
			}
			if test.signature.Router != test.want {
				t.Fatalf("signature router = %q, want %q", test.signature.Router, test.want)
			}
		})
	}
}

func TestSendGroupLateDefaultRouting(t *testing.T) {
	observed := make(chan routingObservation, 2)
	controller := &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
		observed <- routingObservation{id: signature.ID, router: signature.Router}
		return nil
	}}
	server := &Server{
		config:     &Config{ConsumeQueue: "custom-queue"},
		backend:    &groupTestBackend{},
		controller: controller,
	}
	unset := task.NewSignature("unset", "task")
	explicit := task.NewSignature("explicit", "task", task.SetRouter("explicit-queue"))
	group, _ := task.NewGroup("routing-group", "routing", unset, explicit)

	if _, err := server.SendGroupWithContext(context.Background(), group, 2); err != nil {
		t.Fatalf("SendGroupWithContext returned error: %v", err)
	}
	routers := make(map[string]string, 2)
	for i := 0; i < 2; i++ {
		select {
		case observation := <-observed:
			routers[observation.id] = observation.router
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for published router")
		}
	}
	if got := routers["unset"]; got != "custom-queue" {
		t.Fatalf("unset task router = %q, want custom-queue", got)
	}
	if got := routers["explicit"]; got != "explicit-queue" {
		t.Fatalf("explicit task router = %q, want explicit-queue", got)
	}
}
