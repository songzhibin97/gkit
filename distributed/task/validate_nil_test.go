package task

import (
	"errors"
	"testing"
)

// Regression for #166 / 03-03: nil input must be rejected as a non-callable
// task, rather than panicking on reflect.Type or accepting a nil function.
func TestValidateTaskRejectsNilFunctions(t *testing.T) {
	var nilFunc func() error
	for _, test := range []struct {
		name string
		fn   interface{}
	}{
		{name: "typed_nil", fn: nilFunc},
		{name: "nil", fn: nil},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateTask(test.fn); !errors.Is(err, ErrTaskMustFunc) {
				t.Fatalf("ValidateTask(%s) = %v, want %v", test.name, err, ErrTaskMustFunc)
			}
		})
	}
}

func TestValidateTaskPreservesFunctionContracts(t *testing.T) {
	for _, test := range []struct {
		name string
		fn   interface{}
		want error
	}{
		{name: "non_function", fn: 1, want: ErrTaskMustFunc},
		{name: "no_return", fn: func() {}, want: ErrTaskReturnNoValue},
		{name: "no_error", fn: func() int { return 1 }, want: ErrTaskReturnNoErr},
		{name: "error", fn: func() error { return nil }},
		{name: "value_and_error", fn: func() (int, error) { return 1, nil }},
		{name: "concrete_error", fn: func() ErrRetryTaskLater { return ErrRetryTaskLater{} }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateTask(test.fn); err != test.want {
				t.Fatalf("ValidateTask = %v, want %v", err, test.want)
			}
		})
	}
}
