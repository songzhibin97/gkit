package retry

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

// FibonacciNext returns next number in Fibonacci sequence greater than start
func FibonacciNext(start int) int {
	fib := Fibonacci()
	num := fib()
	for num <= start {
		num = fib()
	}
	return num
}
