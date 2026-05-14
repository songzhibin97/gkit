package retry

import "math"

// Fibonacci returns successive Fibonacci numbers starting from 1
func Fibonacci(max ...int) func() int {
	max = append(max, 20)
	a, b := 0, 1
	return func() int {
		if max[0] == 0 {
			return a
		}
		max[0]--
		a, b = b, a+b
		return a
	}
}

// FibonacciNext returns the next Fibonacci number strictly greater than
// start.
//
// The previous implementation reused `Fibonacci()` (default cap = F(20) =
// 6765) and looped `for num <= start { num = fib() }`. For start >= 6765
// the generator saturated and the loop never terminated. Compute the
// sequence inline with an explicit int-overflow guard so callers always
// make forward progress.
func FibonacciNext(start int) int {
	a, b := 1, 1
	for a <= start {
		if b > math.MaxInt/2 {
			// next addition would overflow; clamp.
			if a < start {
				return start + 1
			}
			return a
		}
		a, b = b, a+b
	}
	return a
}
