package hashset

import (
	"math"
	"testing"
)

func TestFloat32SetRejectsNaN(t *testing.T) {
	set := NewFloat32()
	nan := float32(math.NaN())

	if set.Add(nan) {
		t.Fatal("Add(NaN) = true, want false")
	}
	if set.Add(nan) {
		t.Fatal("second Add(NaN) = true, want false")
	}
	if got := set.Len(); got != 0 {
		t.Fatalf("Len() after Add(NaN) = %d, want 0", got)
	}
	if set.Contains(nan) {
		t.Fatal("Contains(NaN) = true, want false")
	}
	if set.Remove(nan) {
		t.Fatal("Remove(NaN) = true, want false")
	}
}

func TestFloat64SetRejectsNaN(t *testing.T) {
	set := NewFloat64()
	nan := math.NaN()

	if set.Add(nan) {
		t.Fatal("Add(NaN) = true, want false")
	}
	if set.Add(nan) {
		t.Fatal("second Add(NaN) = true, want false")
	}
	if got := set.Len(); got != 0 {
		t.Fatalf("Len() after Add(NaN) = %d, want 0", got)
	}
	if set.Contains(nan) {
		t.Fatal("Contains(NaN) = true, want false")
	}
	if set.Remove(nan) {
		t.Fatal("Remove(NaN) = true, want false")
	}
}

func TestFloat32SetPreservesComparableValues(t *testing.T) {
	set := NewFloat32()
	positiveZero := float32(0)
	negativeZero := float32(math.Copysign(0, -1))
	values := []float32{positiveZero, negativeZero, 1.25, float32(math.Inf(1)), float32(math.Inf(-1))}

	for _, value := range values {
		if !set.Add(value) {
			t.Fatalf("Add(%v) = false, want true", value)
		}
		if !set.Contains(value) {
			t.Fatalf("Contains(%v) = false, want true", value)
		}
	}
	if got := set.Len(); got != 4 {
		t.Fatalf("Len() = %d, want 4; +0 and -0 must be the same key", got)
	}
	for _, value := range []float32{negativeZero, 1.25, float32(math.Inf(1)), float32(math.Inf(-1))} {
		if !set.Remove(value) {
			t.Fatalf("Remove(%v) = false, want true", value)
		}
	}
	if got := set.Len(); got != 0 {
		t.Fatalf("Len() after removals = %d, want 0", got)
	}
}

func TestFloat64SetPreservesComparableValues(t *testing.T) {
	set := NewFloat64()
	positiveZero := float64(0)
	negativeZero := math.Copysign(0, -1)
	values := []float64{positiveZero, negativeZero, 1.25, math.Inf(1), math.Inf(-1)}

	for _, value := range values {
		if !set.Add(value) {
			t.Fatalf("Add(%v) = false, want true", value)
		}
		if !set.Contains(value) {
			t.Fatalf("Contains(%v) = false, want true", value)
		}
	}
	if got := set.Len(); got != 4 {
		t.Fatalf("Len() = %d, want 4; +0 and -0 must be the same key", got)
	}
	for _, value := range []float64{negativeZero, 1.25, math.Inf(1), math.Inf(-1)} {
		if !set.Remove(value) {
			t.Fatalf("Remove(%v) = false, want true", value)
		}
	}
	if got := set.Len(); got != 0 {
		t.Fatalf("Len() after removals = %d, want 0", got)
	}
}
