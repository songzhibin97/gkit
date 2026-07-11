package vto

import (
	"errors"
	"testing"

	"github.com/songzhibin97/gkit/tools"
)

func TestCompletionDefaultRejectsInvalidTargets(t *testing.T) {
	type target struct {
		Name string `default:"default-name"`
	}
	var typedNil *target

	for _, tt := range []struct {
		name   string
		target interface{}
	}{
		{name: "untyped-nil", target: nil},
		{name: "typed-nil-pointer", target: typedNil},
		{name: "non-pointer", target: target{}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := CompletionDefault(tt.target); !errors.Is(err, tools.ErrorInvalidValue) {
				t.Fatalf("CompletionDefault error = %v, want %v", err, tools.ErrorInvalidValue)
			}
		})
	}
}

func TestCompletionDefaultValidPointerBehaviorIsUnchanged(t *testing.T) {
	type target struct {
		Name string `default:"default-name"`
	}

	var empty target
	if err := CompletionDefault(&empty); err != nil {
		t.Fatalf("CompletionDefault empty pointer: %v", err)
	}
	if empty.Name != "default-name" {
		t.Fatalf("default Name = %q, want default-name", empty.Name)
	}

	existing := target{Name: "existing"}
	if err := CompletionDefault(&existing); err != nil {
		t.Fatalf("CompletionDefault populated pointer: %v", err)
	}
	if existing.Name != "existing" {
		t.Fatalf("existing Name = %q, want unchanged", existing.Name)
	}
}
