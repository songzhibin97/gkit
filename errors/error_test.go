package errors

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
)

func TestError(t *testing.T) {
	var base *Error
	err := Errorf(http.StatusBadRequest, "reason", "message")
	err2 := Errorf(http.StatusBadRequest, "reason", "message")
	err3 := err.AddMetadata(map[string]string{
		"foo": "bar",
	})
	werr := fmt.Errorf("wrap %w", err)

	if errors.Is(err, new(Error)) {
		t.Errorf("should not be equal: %v", err)
	}
	if !errors.Is(werr, err) {
		t.Errorf("should be equal: %v", err)
	}
	if !errors.Is(werr, err2) {
		t.Errorf("should be equal: %v", err)
	}

	if !errors.As(err, &base) {
		t.Errorf("should be matchs: %v", err)
	}
	if !IsBadRequest(err) {
		t.Errorf("should be matchs: %v", err)
	}

	if reason := Reason(err); reason != err3.Reason {
		t.Errorf("got %s want: %s", reason, err)
	}

	if err3.Metadata["foo"] != "bar" {
		t.Error("not expected metadata")
	}

	gs := err.GRPCStatus()
	se := FromError(gs.Err())
	if se.Reason != se.Reason {
		t.Errorf("got %+v want %+v", se, err)
	}
}

func TestCode(t *testing.T) {
	var (
		input = []error{
			BadRequest("reason_400", "message_400"),
			Unauthorized("reason_401", "message_401"),
			Forbidden("reason_403", "message_403"),
			NotFound("reason_404", "message_404"),
			Conflict("reason_409", "message_409"),
			InternalServer("reason_500", "message_500"),
			ServiceUnavailable("reason_503", "message_503"),
		}
		output = []func(error) bool{
			IsBadRequest,
			IsUnauthorized,
			IsForbidden,
			IsNotFound,
			IsConflict,
			IsInternalServer,
			IsServiceUnavailable,
		}
	)

	for i, in := range input {
		if !output[i](in) {
			t.Errorf("not expect: %v", in)
		}
	}
}
