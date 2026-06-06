package reflect2value

import (
	"errors"
	"math"
	"testing"
)

// TestReflectValue_IntOverflow guards the new narrowing-overflow check: setting
// an int64 into a narrower target (int8/16/32) previously truncated silently
// (e.g. 200 -> -56). It must now return *ErrOverflow instead.
func TestReflectValue_IntOverflow(t *testing.T) {
	v, err := ReflectValue("int8", int64(200))
	if err == nil {
		t.Fatalf("int8=200: want overflow error, got %v", v.Interface())
	}
	var oe *ErrOverflow
	if !errors.As(err, &oe) {
		t.Fatalf("int8=200: want *ErrOverflow, got %T: %v", err, err)
	}

	v, err = ReflectValue("int8", int64(100))
	if err != nil {
		t.Fatalf("int8=100: unexpected error: %v", err)
	}
	if got := v.Int(); got != 100 {
		t.Fatalf("int8=100: got %d, want 100", got)
	}

	// Full-width int64 must not falsely overflow.
	if _, err := ReflectValue("int64", int64(math.MaxInt64)); err != nil {
		t.Fatalf("int64=MaxInt64: unexpected overflow: %v", err)
	}
}

// TestReflectValue_UintOverflow guards the unsigned narrowing check.
func TestReflectValue_UintOverflow(t *testing.T) {
	if _, err := ReflectValue("uint8", uint64(300)); err == nil {
		t.Fatal("uint8=300: want overflow error")
	}
	v, err := ReflectValue("uint8", uint64(200))
	if err != nil {
		t.Fatalf("uint8=200: unexpected error: %v", err)
	}
	if got := v.Uint(); got != 200 {
		t.Fatalf("uint8=200: got %d, want 200", got)
	}
}

// TestReflectValue_FloatOverflow guards the float narrowing check.
func TestReflectValue_FloatOverflow(t *testing.T) {
	if _, err := ReflectValue("float32", float64(1e40)); err == nil {
		t.Fatal("float32=1e40: want overflow error")
	}
	v, err := ReflectValue("float32", float64(1.5))
	if err != nil {
		t.Fatalf("float32=1.5: unexpected error: %v", err)
	}
	if got := v.Float(); got != 1.5 {
		t.Fatalf("float32=1.5: got %v, want 1.5", got)
	}
}

// TestReflectValue_SliceOverflow guards the slice conversion paths, which set
// elements with SetInt/SetUint/SetFloat and previously skipped the overflow
// checks the scalar paths have — so []int8{200} truncated to -56 silently.
func TestReflectValue_SliceOverflow(t *testing.T) {
	if _, err := ReflectValue("[]int8", []int64{200}); err == nil {
		t.Fatal("[]int8 with 200: want overflow error")
	}
	if _, err := ReflectValue("[]uint8", []uint64{300}); err == nil {
		t.Fatal("[]uint8 with 300: want overflow error")
	}
	if _, err := ReflectValue("[]float32", []float64{1e40}); err == nil {
		t.Fatal("[]float32 with 1e40: want overflow error")
	}

	// In-range slices still convert.
	v, err := ReflectValue("[]int8", []int64{100, 127})
	if err != nil {
		t.Fatalf("[]int8 in-range: unexpected error: %v", err)
	}
	if v.Len() != 2 || v.Index(0).Int() != 100 || v.Index(1).Int() != 127 {
		t.Fatalf("[]int8 in-range: got %v", v.Interface())
	}
}
