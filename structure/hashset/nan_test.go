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

func TestComplex64SetRejectsNaNComponents(t *testing.T) {
	set := NewComplex64()
	valid := complex(float32(1), float32(2))
	if !set.Add(valid) {
		t.Fatal("Add(valid) = false, want true")
	}
	nan := float32(math.NaN())
	values := []struct {
		name  string
		value complex64
	}{
		{name: "real", value: complex(nan, 1)},
		{name: "imaginary", value: complex(1, nan)},
		{name: "both", value: complex(nan, nan)},
	}

	for _, tt := range values {
		t.Run(tt.name, func(t *testing.T) {
			before := set.Len()
			if set.Add(tt.value) {
				t.Fatal("Add(complex NaN) = true, want false")
			}
			if got := set.Len(); got != before {
				t.Fatalf("Len() after Add(complex NaN) = %d, want %d", got, before)
			}
			if set.Contains(tt.value) {
				t.Fatal("Contains(complex NaN) = true, want false")
			}
			if set.Remove(tt.value) {
				t.Fatal("Remove(complex NaN) = true, want false")
			}
			if got := set.Len(); got != before {
				t.Fatalf("Len() after Remove(complex NaN) = %d, want %d", got, before)
			}
		})
	}
	if !set.Contains(valid) || set.Len() != 1 {
		t.Fatalf("valid value changed while rejecting NaN: contains=%t len=%d", set.Contains(valid), set.Len())
	}
}

func TestComplex128SetRejectsNaNComponents(t *testing.T) {
	set := NewComplex128()
	valid := complex(1.0, 2.0)
	if !set.Add(valid) {
		t.Fatal("Add(valid) = false, want true")
	}
	nan := math.NaN()
	values := []struct {
		name  string
		value complex128
	}{
		{name: "real", value: complex(nan, 1)},
		{name: "imaginary", value: complex(1, nan)},
		{name: "both", value: complex(nan, nan)},
	}

	for _, tt := range values {
		t.Run(tt.name, func(t *testing.T) {
			before := set.Len()
			if set.Add(tt.value) {
				t.Fatal("Add(complex NaN) = true, want false")
			}
			if got := set.Len(); got != before {
				t.Fatalf("Len() after Add(complex NaN) = %d, want %d", got, before)
			}
			if set.Contains(tt.value) {
				t.Fatal("Contains(complex NaN) = true, want false")
			}
			if set.Remove(tt.value) {
				t.Fatal("Remove(complex NaN) = true, want false")
			}
			if got := set.Len(); got != before {
				t.Fatalf("Len() after Remove(complex NaN) = %d, want %d", got, before)
			}
		})
	}
	if !set.Contains(valid) || set.Len() != 1 {
		t.Fatalf("valid value changed while rejecting NaN: contains=%t len=%d", set.Contains(valid), set.Len())
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
