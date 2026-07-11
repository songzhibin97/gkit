package float

import (
	"math"
	"testing"
)

func TestTruncFloatExtremePrecision(t *testing.T) {
	tests := []struct {
		name string
		f    float64
		prec int
		want float64
	}{
		{name: "positive scale overflow", f: math.MaxFloat64, prec: 1, want: math.MaxFloat64},
		{name: "negative scale overflow", f: -math.MaxFloat64, prec: 1, want: -math.MaxFloat64},
		{name: "positive infinite scale", f: 1.25, prec: 400, want: 1.25},
		{name: "negative infinite scale", f: -1.25, prec: 400, want: -1.25},
		{name: "positive zero infinite scale", f: 0, prec: 400, want: 0},
		{name: "negative zero infinite scale", f: math.Copysign(0, -1), prec: 400, want: math.Copysign(0, -1)},
		{name: "positive zero scale", f: 1.25, prec: -400, want: 0},
		{name: "negative zero scale", f: -1.25, prec: -400, want: math.Copysign(0, -1)},
	}

	for _, tt := range tests {
		current := tt
		t.Run(current.name, func(t *testing.T) {
			got := TruncFloat(current.f, current.prec)
			if got != current.want || math.Signbit(got) != math.Signbit(current.want) {
				t.Fatalf("TruncFloat(%v, %d) = %v, want %v", current.f, current.prec, got, current.want)
			}
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatalf("finite input TruncFloat(%v, %d) returned non-finite %v", current.f, current.prec, got)
			}
		})
	}
}

func TestTruncFloatRegularPrecision(t *testing.T) {
	tests := []struct {
		f    float64
		prec int
		want float64
	}{
		{f: 1.239, prec: 2, want: 1.23},
		{f: -1.239, prec: 2, want: -1.23},
		{f: 123.9, prec: -1, want: 120},
		{f: -123.9, prec: -1, want: -120},
		{f: 12.9, prec: 0, want: 12},
		{f: -12.9, prec: 0, want: -12},
	}

	for _, tt := range tests {
		if got := TruncFloat(tt.f, tt.prec); got != tt.want {
			t.Errorf("TruncFloat(%v, %d) = %v, want %v", tt.f, tt.prec, got, tt.want)
		}
	}
}

func TestTruncFloatFiniteInputsRemainFinite(t *testing.T) {
	inputs := []float64{
		0, math.Copysign(0, -1),
		math.SmallestNonzeroFloat64, -math.SmallestNonzeroFloat64,
		1.25, -1.25, math.MaxFloat64, -math.MaxFloat64,
	}
	precisions := []int{-1000, -400, -324, -323, -309, -308, -1, 0, 1, 308, 309, 400, 1000}

	for _, f := range inputs {
		for _, prec := range precisions {
			got := TruncFloat(f, prec)
			if math.IsNaN(got) || math.IsInf(got, 0) {
				t.Fatalf("finite input TruncFloat(%v, %d) returned non-finite %v", f, prec, got)
			}
		}
	}
}

func TestTruncFloatNonFiniteInput(t *testing.T) {
	for _, prec := range []int{-400, 2, 400} {
		if got := TruncFloat(math.Inf(1), prec); !math.IsInf(got, 1) {
			t.Errorf("TruncFloat(+Inf, %d) = %v, want +Inf", prec, got)
		}
		if got := TruncFloat(math.Inf(-1), prec); !math.IsInf(got, -1) {
			t.Errorf("TruncFloat(-Inf, %d) = %v, want -Inf", prec, got)
		}
		if got := TruncFloat(math.NaN(), prec); !math.IsNaN(got) {
			t.Errorf("TruncFloat(NaN, %d) = %v, want NaN", prec, got)
		}
	}
}
