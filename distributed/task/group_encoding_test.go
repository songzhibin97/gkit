package task

import (
	"reflect"
	"testing"
)

const testStringSliceJSONPrefix = "gkit:string-slice:v1:"

func TestStringSliceValuePreservesLegacyEncodingForOrdinaryIDs(t *testing.T) {
	got, err := (StringSlice{"task-a", "task-b"}).Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if got != "task-a,task-b" {
		t.Fatalf("Value = %#v, want legacy bytes %q", got, "task-a,task-b")
	}
}

func TestStringSliceVersionedRoundTripForCommaID(t *testing.T) {
	want := StringSlice{"task,one", "task-two"}
	encoded, err := want.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if encoded != testStringSliceJSONPrefix+`["task,one","task-two"]` {
		t.Fatalf("Value = %#v, want versioned JSON", encoded)
	}

	var got StringSlice
	if err := got.Scan(encoded); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func TestStringSliceVersionedRoundTripForMarkerCollision(t *testing.T) {
	want := StringSlice{testStringSliceJSONPrefix + `["literal"]`}
	encoded, err := want.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if encoded == want[0] {
		t.Fatal("marker-looking legacy value was not escaped into the versioned format")
	}

	var got StringSlice
	if err := got.Scan(encoded); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %#v, want %#v", got, want)
	}
}

func TestStringSliceScanLegacyDriverRepresentations(t *testing.T) {
	for _, src := range []interface{}{[]byte("task-a,task-b"), "task-a,task-b"} {
		var got StringSlice
		if err := got.Scan(src); err != nil {
			t.Fatalf("Scan(%T): %v", src, err)
		}
		want := StringSlice{"task-a", "task-b"}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Scan(%T) = %#v, want %#v", src, got, want)
		}
	}

	got := StringSlice{"stale"}
	if err := got.Scan(nil); err != nil {
		t.Fatalf("Scan(nil): %v", err)
	}
	if got != nil {
		t.Fatalf("Scan(nil) = %#v, want nil", got)
	}
}

func TestStringSliceScanRejectsMalformedVersionedPayload(t *testing.T) {
	for _, payload := range []string{`{`, `null`, `{}`, `"task"`, `[null]`, `["ok",null]`, `[1]`} {
		got := StringSlice{"unchanged"}
		if err := got.Scan(testStringSliceJSONPrefix + payload); err == nil {
			t.Fatalf("Scan accepted invalid versioned payload %q", payload)
		}
		if !reflect.DeepEqual(got, StringSlice{"unchanged"}) {
			t.Fatalf("failed Scan(%q) mutated receiver to %#v", payload, got)
		}
	}
}
