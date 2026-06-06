package mbuffer

import "testing"

// TestFree_EmptySlice covers C19: Free(make([]byte, 0)) previously
// panicked because isPowerOfTwo(0) was true and bsr(0) == -1 indexed
// caches[-1].
func TestFree_EmptySlice(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Free(zero-cap slice) panicked: %v", r)
		}
	}()
	Free(make([]byte, 0))
	Free(nil)
}

// TestMalloc_NegativeRejected covers I-f: Malloc(-1) previously indexed
// caches at an OOB position via uint(-1) → bits.Len → calcIndex. Now it
// panics cleanly with a recognizable message.
func TestMalloc_NegativeRejected(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Malloc(-1) did not panic as expected")
		}
	}()
	_ = Malloc(-1)
}
