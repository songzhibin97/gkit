package retry

import (
	"testing"
	"time"
)

// TestFibonacciNext_NoInfiniteLoopAtSaturation covers I-x: previously
// `FibonacciNext(>=F(20))` looped forever because the internal sequence
// saturated at F(20)=6765 and `for num <= start` never became false.
func TestFibonacciNext_NoInfiniteLoopAtSaturation(t *testing.T) {
	done := make(chan int, 1)
	go func() {
		done <- FibonacciNext(10_000)
	}()
	select {
	case v := <-done:
		if v <= 10_000 {
			t.Fatalf("FibonacciNext(10000) = %d, expected > start", v)
		}
	case <-time.After(time.Second):
		t.Fatal("FibonacciNext hung — saturation handling regressed")
	}
}

func TestFibonacciNext_NormalPath(t *testing.T) {
	if got := FibonacciNext(0); got != 1 {
		t.Fatalf("FibonacciNext(0) = %d, want 1", got)
	}
	if got := FibonacciNext(8); got != 13 {
		t.Fatalf("FibonacciNext(8) = %d, want 13", got)
	}
}
