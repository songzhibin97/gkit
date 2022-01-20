package retry

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFibonacci(t *testing.T) {
	fibonacci := Fibonacci()

	sequence := []int{
		fibonacci(),
		fibonacci(),
		fibonacci(),
		fibonacci(),
		fibonacci(),
		fibonacci(),
	}

	assert.EqualValues(t, sequence, []int{1, 1, 2, 3, 5, 8})
}

func TestFibonacciNext(t *testing.T) {
	assert.Equal(t, 1, FibonacciNext(0))
	assert.Equal(t, 2, FibonacciNext(1))
	assert.Equal(t, 5, FibonacciNext(3))
	assert.Equal(t, 5, FibonacciNext(4))
	assert.Equal(t, 8, FibonacciNext(5))
	assert.Equal(t, 13, FibonacciNext(8))
}
