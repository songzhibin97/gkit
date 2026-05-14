package buffer

import "testing"

// TestBytePool_BoundarySizesNoPanic covers C18: sizes in (1<<17, 1<<18]
// previously panicked because slot() returned an index one past the last
// allocated pool bucket.
func TestBytePool_BoundarySizesNoPanic(t *testing.T) {
	for _, sz := range []int{1, 64, 65, 4096, 1 << 17, (1 << 17) + 1, (1 << 18) - 1, 1 << 18} {
		buf := GetBytes(sz)
		if buf == nil || len(*buf) != sz {
			t.Fatalf("GetBytes(%d) len = %v", sz, buf)
		}
		PutBytes(buf)
	}
}
