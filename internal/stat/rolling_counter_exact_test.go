package stat

import (
	"math"
	"sync"
	"testing"
	"time"
)

func TestRollingCounterExactValue(t *testing.T) {
	for _, val := range []int64{0, 1, 1<<53 - 1, 1 << 53, 1<<53 + 1, math.MaxInt64} {
		r := NewRollingCounter(3, time.Hour)
		r.Add(val)
		if got := r.Value(); got != val {
			t.Errorf("Add(%d): Value()=%d", val, got)
		}
	}
	r := NewRollingCounter(3, time.Hour)
	r.Add(1 << 53)
	r.Add(1)
	if got := r.Value(); got != 1<<53+1 {
		t.Fatalf("repeated Add Value()=%d, want %d", got, int64(1<<53+1))
	}
}

func advanceCounter(r *rollingCounter, buckets int) {
	r.policy.mu.Lock()
	r.policy.lastAppendTime = r.policy.lastAppendTime.Add(-time.Duration(buckets) * r.policy.bucketDuration)
	r.policy.mu.Unlock()
}

func TestRollingCounterExactValueExpiry(t *testing.T) {
	r := NewRollingCounter(3, time.Hour).(*rollingCounter)
	const big = int64(1<<53 + 1)
	check := func(want int64) {
		t.Helper()
		if got := r.Value(); got != want {
			t.Fatalf("Value()=%d, want %d", got, want)
		}
	}
	r.Add(big)
	check(big)
	advanceCounter(r, 1)
	r.Add(3)
	check(big + 3)
	advanceCounter(r, 1)
	r.Add(5)
	check(big + 8)
	advanceCounter(r, 1)
	check(8)
	r.Add(7)
	check(15)
	advanceCounter(r, 3)
	check(0)
	r.Add(9)
	check(9)
	advanceCounter(r, 5)
	check(0)
	r.Add(11)
	check(11)
	r.policy.mu.Lock()
	r.policy.window.ResetWindow()
	r.policy.mu.Unlock()
	check(0)
}

func TestRollingCounterExactValueConcurrent(t *testing.T) {
	r := NewRollingCounter(3, time.Hour)
	const big = int64(1<<53 + 1)
	r.Add(big)
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				r.Add(1)
				if got := r.Value(); got < big || got > big+4000 {
					t.Errorf("concurrent Value=%d", got)
					return
				}
			}
		}()
	}
	wg.Wait()
	if got := r.Value(); got != big+4000 {
		t.Fatalf("Value()=%d, want %d", got, big+4000)
	}
}

func TestRollingCounterExactValueReentrantReduce(t *testing.T) {
	r := NewRollingCounter(3, time.Hour)
	const big = int64(1<<53 + 1)
	r.Add(big)
	got := r.Reduce(func(i Iterator) float64 {
		r.Add(2)
		if value := r.Value(); value != big+2 {
			t.Errorf("reentrant Value()=%d, want %d", value, big+2)
		}
		return Sum(i)
	})
	if got != float64(big) {
		t.Fatalf("snapshot Sum=%v, want %v", got, float64(big))
	}
	if r.Value() != big+2 {
		t.Fatalf("Value()=%d, want %d", r.Value(), big+2)
	}
	if r.Sum() != float64(big)+2 {
		t.Fatalf("float Sum=%v", r.Sum())
	}
}
