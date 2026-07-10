package xml

import "testing"

type issue84Payload struct {
	Value string `xml:"value"`
}

func TestUnmarshalTypedNilTarget(t *testing.T) {
	var target *issue84Payload
	if err := (code{}).Unmarshal([]byte(`<payload><value>decoded</value></payload>`), target); err == nil {
		t.Fatal("Unmarshal() error = nil, want non-nil typed target error")
	}
	if target != nil {
		t.Fatalf("Unmarshal() changed typed-nil target to %#v", target)
	}
}

func TestUnmarshalAllocatesNestedPointerTarget(t *testing.T) {
	var target *issue84Payload
	if err := (code{}).Unmarshal([]byte(`<payload><value>decoded</value></payload>`), &target); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if target == nil || target.Value != "decoded" {
		t.Fatalf("Unmarshal() target = %#v, want allocated decoded payload", target)
	}
}
