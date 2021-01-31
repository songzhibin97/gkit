package errors

import (
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"testing"
)

func TestIsError(t *testing.T) {
	s1 := &ErrorCode{Code: 1}
	s2 := &ErrorCode{Code: 2}
	if errors.Is(s1, s2) {
		t.Errorf("error is not equal: %+v -> %+v", s1, s2)
	}
	s1.Code = 1
	s2.Code = 1
	if !errors.Is(s1, s2) {
		t.Errorf("error is not equal: %+v -> %+v", s1, s2)
	}

	s1.Reason = "test"
	s2.Reason = "test"

	if !errors.Is(s1, s2) {
		t.Errorf("error is not equal: %+v -> %+v", s1, s2)
	}

	if ErrToReason(s1) != "test" {
		t.Errorf("error is not equal: %+v", s1)
	}

	if ErrToReason(s2) != "test" {
		t.Errorf("error is not equal: %+v", s1)
	}

	err1 := &ErrorCode{Code: 1}
	err2 := fmt.Errorf("warp err %w", err1)
	if !errors.Is(err2, err1) {
		t.Errorf("error is not equal: a: %v b: %v ", err2, err1)
	}
}

func TestErrorAs(t *testing.T) {
	err1 := &ErrorCode{Code: 1}
	err2 := fmt.Errorf("wrap : %w", err1)

	err3 := new(ErrorCode)
	if !errors.As(err2, &err3) {
		t.Errorf("error is not equal: %v", err2)
	}
}

func TestAddDetails(t *testing.T) {
	details := []proto.Message{
		&errdetails.ErrorInfo{
			Reason:   "reason",
			Metadata: map[string]string{"message": "message"},
		},
	}
	err1 := &ErrorCode{Code: 0}
	err := err1.AddDetails(details...)
	if !errors.Is(err, ErrDetails) {
		t.Errorf("error is not equal: a: %v b: %v ", err, ErrDetails)
	}
	err2 := &ErrorCode{Code: 1}
	err = err2.AddDetails(details...)
	if err != nil {
		t.Errorf("error is not nil %v", err)
	}
	t.Log(err2)
}
