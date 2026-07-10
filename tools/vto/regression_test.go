package vto

import (
	"errors"
	"testing"

	"github.com/songzhibin97/gkit/tools"
)

type issue84Destination struct {
	Count int
}

type issue84Source struct {
	Count *int
}

func TestVoToDoSkipsNilSourcePointerField(t *testing.T) {
	dst := issue84Destination{Count: 42}
	src := issue84Source{}

	if err := VoToDo(&dst, &src); err != nil {
		t.Fatalf("VoToDo: %v", err)
	}
	if dst.Count != 42 {
		t.Fatalf("Count = %d, want existing value 42", dst.Count)
	}
}

func TestVoToDoPlusFieldBindSkipsNilSourcePointerField(t *testing.T) {
	dst := issue84Destination{Count: 42}
	src := issue84Source{}

	if err := VoToDoPlus(&dst, &src, ModelParameters{Model: FieldBind}); err != nil {
		t.Fatalf("VoToDoPlus: %v", err)
	}
	if dst.Count != 42 {
		t.Fatalf("Count = %d, want existing value 42", dst.Count)
	}
}

func TestVoToDoNilTopLevelPointerReturnsExistingError(t *testing.T) {
	var dst *issue84Destination
	src := issue84Source{}

	err := VoToDo(dst, &src)
	if !errors.Is(err, tools.ErrorInvalidValue) {
		t.Fatalf("VoToDo error = %v, want %v", err, tools.ErrorInvalidValue)
	}
}
