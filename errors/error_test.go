package errors

import (
	"errors"
	"fmt"
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
