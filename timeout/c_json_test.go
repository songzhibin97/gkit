package timeout

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestDBJSONFromObjectEReturnsMarshalError(t *testing.T) {
	got, err := DBJSONFromObjectE(make(chan int))
	if err == nil {
		t.Fatal("DBJSONFromObjectE returned nil error for an unsupported channel")
	}
	if got != nil {
		t.Fatalf("DBJSONFromObjectE returned %q on error, want nil", got)
	}
	var typeErr *json.UnsupportedTypeError
	if !errors.As(err, &typeErr) {
		t.Fatalf("DBJSONFromObjectE error = %T %v, want json.UnsupportedTypeError", err, err)
	}
	if !strings.HasPrefix(err.Error(), "timeout: marshal DbJSON: ") {
		t.Fatalf("DBJSONFromObjectE error = %q, want timeout marshal context", err)
	}
}

func TestDBJSONFromObjectEValidAndNullSemantics(t *testing.T) {
	got, err := DBJSONFromObjectE(map[string]int{"answer": 42})
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"answer":42}` {
		t.Fatalf("DBJSONFromObjectE object = %q, want compact JSON", got)
	}
	value, err := got.Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != `{"answer":42}` {
		t.Fatalf("DbJSON.Value() = %#v, want JSON string", value)
	}

	nullJSON, err := DBJSONFromObjectE(nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(nullJSON) != "null" {
		t.Fatalf("DBJSONFromObjectE(nil) = %q, want null", nullJSON)
	}
	nullValue, err := nullJSON.Value()
	if err != nil {
		t.Fatal(err)
	}
	if nullValue != nil {
		t.Fatalf("DbJSON(null).Value() = %#v, want SQL NULL", nullValue)
	}
}

func TestDBJSONFromObjectPreservesLegacyFallback(t *testing.T) {
	legacyNull := DBJSONFromObject(make(chan int))
	if legacyNull != nil {
		t.Fatalf("DBJSONFromObject(channel) = %q, want legacy nil fallback", legacyNull)
	}
	value, err := legacyNull.Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != nil {
		t.Fatalf("legacy nil fallback Value() = %#v, want SQL NULL", value)
	}

	want, err := DBJSONFromObjectE(map[string]bool{"ok": true})
	if err != nil {
		t.Fatal(err)
	}
	if got := DBJSONFromObject(map[string]bool{"ok": true}); string(got) != string(want) {
		t.Fatalf("DBJSONFromObject(valid) = %q, want %q", got, want)
	}
}
