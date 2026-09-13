package task

import (
	"errors"
	"testing"
)

func TestTaskCallPreservesCompletionAndPanicErrors(t *testing.T) {
	want := errors.New("controlled task error")
	for _, tc := range []struct {
		name       string
		fn         interface{}
		wantErr    error
		panicError bool
		message    string
		wrapped    bool
	}{
		{name: "success", fn: func() (string, error) { return "completed", nil }},
		{name: "returned_error", fn: func() error { return want }, wantErr: want},
		{name: "error_panic", fn: func() error { panic(want) }, wantErr: want},
		{name: "nil_panic", fn: func() error { panic(nil) }, panicError: true},
		{name: "string_panic", fn: func() error { panic("controlled string panic") }, panicError: true, message: "controlled string panic"},
		{name: "value_panic", fn: func() error { panic(42) }, panicError: true, wrapped: true},
		{name: "no_return", fn: func() {}, wantErr: ErrTaskReturnNoValue},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call, err := NewTask(tc.fn, nil)
			if err != nil {
				t.Fatal(err)
			}
			results, err := call.Call()
			if tc.wantErr != nil {
				if err != tc.wantErr {
					t.Fatalf("error = %v, want original %v", err, tc.wantErr)
				}
			} else if tc.panicError {
				if err == nil {
					t.Fatal("panicked task returned a nil error")
				}
				if tc.message != "" && err.Error() != tc.message {
					t.Fatalf("panic message = %q, want %q", err.Error(), tc.message)
				}
				if tc.wrapped && !errors.Is(err, ErrDispatching) {
					t.Fatalf("panic error %v does not wrap ErrDispatching", err)
				}
			} else {
				if err != nil || len(results) != 1 || results[0].Type != "string" || results[0].Value != "completed" {
					t.Fatalf("normal result = %v, error = %v", results, err)
				}
			}
			if err != nil && len(results) != 0 {
				t.Fatalf("failed invocation returned success results: %v", results)
			}
		})
	}
}
