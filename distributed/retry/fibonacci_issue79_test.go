package retry

import (
	"math"
	"testing"
)

func TestFibonacciNextSaturatesAtMaxInt(t *testing.T) {
	if got := FibonacciNext(math.MaxInt); got != math.MaxInt {
		t.Fatalf("FibonacciNext(MaxInt) = %d, want saturated MaxInt %d", got, math.MaxInt)
	}
	if got := FibonacciNext(math.MaxInt - 1); got != math.MaxInt {
		t.Fatalf("FibonacciNext(MaxInt-1) = %d, want MaxInt %d", got, math.MaxInt)
	}
}
