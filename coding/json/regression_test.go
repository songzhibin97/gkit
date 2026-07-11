package json

import (
	"strings"
	"testing"
)

type issue84Payload struct {
	Value string `json:"value"`
}

func TestUnmarshalTypedNilTarget(t *testing.T) {
	var target *issue84Payload
	err := (code{}).Unmarshal([]byte(`{"value":"decoded"}`), target)
	if err == nil {
		t.Fatal("Unmarshal() error = nil, want non-nil typed target error")
	}
	if !strings.Contains(err.Error(), "json: unmarshal target is a nil *json.issue84Payload") {
		t.Fatalf("Unmarshal() error = %q, want explicit typed-nil target error", err)
	}
	if target != nil {
		t.Fatalf("Unmarshal() changed typed-nil target to %#v", target)
	}
}

func TestUnmarshalUntypedNilTarget(t *testing.T) {
	err := (code{}).Unmarshal([]byte(`{"value":"decoded"}`), nil)
	if err == nil {
		t.Fatal("Unmarshal() error = nil, want untyped-nil target error")
	}
	if !strings.Contains(err.Error(), "json: unmarshal target is nil") {
		t.Fatalf("Unmarshal() error = %q, want explicit untyped-nil target error", err)
	}
}

func TestUnmarshalAllocatesNestedPointerTarget(t *testing.T) {
	var target *issue84Payload
	if err := (code{}).Unmarshal([]byte(`{"value":"decoded"}`), &target); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if target == nil || target.Value != "decoded" {
		t.Fatalf("Unmarshal() target = %#v, want allocated decoded payload", target)
	}
}
