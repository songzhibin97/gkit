package task

import (
	"errors"
	"testing"
	"time"
)

type concreteTaskError struct{ message string }

func (e concreteTaskError) Error() string { return e.message }

type concreteTaskCode int

func (e concreteTaskCode) Error() string { return "synthetic task code" }

type customTaskRetry struct{ delay time.Duration }

func (e customTaskRetry) Error() string          { return "synthetic custom retry" }
func (e customTaskRetry) RetryIn() time.Duration { return e.delay }

// Regression for #166 / 03-05: return types accepted by ValidateTask must be
// returned as their actual error, not a reflection/type-assertion failure.
func TestTaskCallPreservesConcreteErrors(t *testing.T) {
	plain := concreteTaskError{message: "synthetic concrete failure"}
	code := concreteTaskCode(7)
	custom := customTaskRetry{delay: time.Minute}
	standard := NewErrRetryTaskLater("synthetic retry", time.Minute)
	for _, test := range []struct {
		name string
		fn   interface{}
		want error
	}{
		{"struct", func() concreteTaskError { return plain }, plain},
		{"integer", func() concreteTaskCode { return code }, code},
		{"pointer", func() *concreteTaskError { return &plain }, &plain},
		{"interface_typed_nil", func() error { return (*concreteTaskError)(nil) }, (*concreteTaskError)(nil)},
		{"standard_retry", func() ErrRetryTaskLater { return standard }, standard},
		{"custom_retry", func() customTaskRetry { return custom }, custom},
		{"custom_retry_pointer", func() *customTaskRetry { return &custom }, &custom},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateTask(test.fn); err != nil {
				t.Fatalf("validator rejected supported error: %v", err)
			}
			exec, err := NewTask(test.fn, nil)
			if err != nil {
				t.Fatal(err)
			}
			results, err := exec.Call()
			if results != nil || !errors.Is(err, test.want) {
				t.Fatalf("Call = (%v, %T %v), want original %T %v", results, err, err, test.want, test.want)
			}
			if want, ok := test.want.(Retrievable); ok {
				var got Retrievable
				if !errors.As(err, &got) || got.RetryIn() != want.RetryIn() {
					t.Fatalf("retry error/delay lost: %v", err)
				}
			}
		})
	}
}

func TestTaskCallKeepsNilErrorResults(t *testing.T) {
	for _, test := range []struct {
		name string
		fn   interface{}
	}{
		{"interface", func() (string, error) { return "result", nil }},
		{"concrete_pointer", func() (string, *concreteTaskError) { return "result", nil }},
	} {
		t.Run(test.name, func(t *testing.T) {
			if err := ValidateTask(test.fn); err != nil {
				t.Fatal(err)
			}
			exec, err := NewTask(test.fn, nil)
			if err != nil {
				t.Fatal(err)
			}
			results, err := exec.Call()
			if err != nil || len(results) != 1 || results[0].Type != "string" || results[0].Value != "result" {
				t.Fatalf("nil-error Call lost result: %v, %v", results, err)
			}
		})
	}
}
