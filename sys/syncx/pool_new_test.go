package syncx

import "testing"

func TestPoolSequentialNewChanges(t *testing.T) {
	var pool Pool
	if got := pool.Get(); got != nil {
		t.Fatalf("zero pool Get = %v, want nil", got)
	}
	for _, want := range []int{97, 98} {
		calls := 0
		pool.New = func() interface{} { calls++; return want }
		for i := 1; i <= 2; i++ {
			if got := pool.Get(); got != want || calls != i {
				t.Fatalf("Get = %v, calls = %d; want %d, %d", got, calls, want, i)
			}
		}
	}
	pool.New = nil
	if got := pool.Get(); got != nil {
		t.Fatalf("Get after clearing New = %v, want nil", got)
	}
	pool.New = func() interface{} { return nil }
	if got := pool.Get(); got != nil {
		t.Fatalf("nil constructor Get = %v, want nil", got)
	}
}
