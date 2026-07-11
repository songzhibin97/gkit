package proto

import (
	"strings"
	"testing"

	"github.com/songzhibin97/gkit/coding"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestRegisteredCodeUnmarshalRejectsTypedNilMessage(t *testing.T) {
	codec := coding.GetCode(Name)
	if codec == nil {
		t.Fatal("registered proto codec is nil")
	}

	var target *emptypb.Empty
	err, panicValue := unmarshalWithoutPanicking(codec, nil, target)
	if panicValue != nil {
		t.Fatalf("Unmarshal() panicked for typed-nil proto.Message: %v", panicValue)
	}
	if err == nil {
		t.Fatal("Unmarshal() error = nil, want typed-nil target error")
	}
	if want := "proto: unmarshal target is a nil *emptypb.Empty"; !strings.Contains(err.Error(), want) {
		t.Fatalf("Unmarshal() error = %q, want it to contain %q", err, want)
	}
}

func TestRegisteredCodeNonNilRoundTrip(t *testing.T) {
	codec := coding.GetCode(Name)
	want := wrapperspb.String("decoded")
	data, err := codec.Marshal(want)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	got := new(wrapperspb.StringValue)
	if err := codec.Unmarshal(data, got); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if got.Value != want.Value {
		t.Fatalf("round trip value = %q, want %q", got.Value, want.Value)
	}
}

func unmarshalWithoutPanicking(codec coding.Code, data []byte, target interface{}) (err error, panicValue interface{}) {
	defer func() { panicValue = recover() }()
	err = codec.Unmarshal(data, target)
	return err, nil
}
