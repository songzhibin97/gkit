package stat

import (
	"sync"
	"testing"
	"time"
)

func TestReducePanicDoesNotPoisonPolicyLock(t *testing.T) {
	policy := NewRollingPolicy(NewWindow(3), time.Hour)
	policy.Add(1)
	wantPanic := &struct{}{}
	gotPanic := func() (recovered interface{}) {
		defer func() { recovered = recover() }()
		policy.Reduce(func(Iterator) float64 { panic(wantPanic) })
		return nil
	}()
	if gotPanic != wantPanic {
		t.Fatalf("recovered panic = %v, want original panic %v", gotPanic, wantPanic)
	}

	added := make(chan struct{})
	go func() {
		policy.Add(1)
		close(added)
	}()
	select {
	case <-added:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Add remained blocked after Reduce callback panic")
	}
}

func TestReduceCallbackCanReenterAdd(t *testing.T) {
	policy := NewRollingPolicy(NewWindow(3), time.Hour)
	policy.Add(1)
	done := make(chan float64, 1)
	go func() {
		done <- policy.Reduce(func(iterator Iterator) float64 {
			policy.Add(2)
			return Sum(iterator)
		})
	}()

	select {
	case got := <-done:
		if got != 1 {
			t.Fatalf("snapshot sum = %v, want 1", got)
		}
	case <-time.After(250 * time.Millisecond):
		t.Fatal("Reduce callback deadlocked while reentering Add")
	}
	if got := policy.Reduce(Sum); got != 3 {
		t.Fatalf("sum after reentrant Add = %v, want 3", got)
	}
}

func TestReduceSnapshotPreservesOrderAndOwnsPoints(t *testing.T) {
	window := NewWindow(3)
	window.Append(0, 10)
	window.Append(0, 11)
	window.Append(1, 20)
	window.Append(2, 30)
	policy := NewRollingPolicy(window, time.Hour)

	var gotPoints [][]float64
	var gotCounts []int64
	gotResult := policy.Reduce(func(iterator Iterator) float64 {
		for iterator.Next() {
			bucket := iterator.Bucket()
			gotPoints = append(gotPoints, append([]float64(nil), bucket.Points...))
			gotCounts = append(gotCounts, bucket.Count)
			if len(bucket.Points) > 0 {
				bucket.Points[0] = -1
			}
		}
		return 42
	})

	if gotResult != 42 {
		t.Fatalf("Reduce result = %v, want 42", gotResult)
	}
	wantPoints := [][]float64{{20}, {30}, {10, 11}}
	if !equalPointSlices(gotPoints, wantPoints) {
		t.Fatalf("snapshot points = %v, want %v", gotPoints, wantPoints)
	}
	wantCounts := []int64{1, 1, 2}
	for i, want := range wantCounts {
		if gotCounts[i] != want {
			t.Fatalf("snapshot count[%d] = %d, want %d", i, gotCounts[i], want)
		}
	}
	for i, want := range [][]float64{{10, 11}, {20}, {30}} {
		if got := window.Bucket(i).Points; !equalPoints(got, want) {
			t.Fatalf("live bucket %d points = %v, want %v", i, got, want)
		}
	}
}

func TestReduceSnapshotConcurrentAdd(t *testing.T) {
	policy := NewRollingPolicy(NewWindow(4), time.Hour)
	policy.Add(1)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				policy.Add(1)
			}
		}()
	}
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 1000; j++ {
				policy.Reduce(func(iterator Iterator) float64 {
					var total float64
					for iterator.Next() {
						bucket := iterator.Bucket()
						for _, point := range bucket.Points {
							total += point
						}
					}
					return total
				})
			}
		}()
	}
	wg.Wait()
}

func equalPointSlices(got, want [][]float64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if !equalPoints(got[i], want[i]) {
			return false
		}
	}
	return true
}

func equalPoints(got, want []float64) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
