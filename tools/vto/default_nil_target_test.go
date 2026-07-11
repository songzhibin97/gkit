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
		name    string
		target  interface{}
		wantErr error
	}{
		{name: "untyped-nil", target: nil, wantErr: tools.ErrorInvalidValue},
		{name: "typed-nil-pointer", target: typedNil, wantErr: tools.ErrorInvalidValue},
		{name: "non-pointer", target: target{}, wantErr: tools.ErrorMustPtr},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := CompletionDefault(tt.target)
			if !errors.Is(err, tt.wantErr) {
				t.Fatalf("CompletionDefault error = %v, want errors.Is(_, %v)", err, tt.wantErr)
			}
			if err != tt.wantErr {
				t.Fatalf("CompletionDefault error = %v, want exact sentinel %v", err, tt.wantErr)
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
