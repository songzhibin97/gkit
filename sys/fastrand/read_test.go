package fastrand

import (
	"testing"
)

// TestRead_UnalignedDoesNotCrash exercises Read with backing arrays at
// every offset 0..7 inside a larger array — at least one of which is
// guaranteed to be 8-byte unaligned. The previous implementation
// reinterpreted []byte as []uint64 via unsafe.Pointer; on strict-
// alignment arches (arm64/mips) the unaligned 8-byte store would trap.
// Even on amd64 the slice-header reinterpret is a `go vet` violation.
func TestRead_UnalignedDoesNotCrash(t *testing.T) {
	buf := make([]byte, 1024)
	for off := 0; off < 8; off++ {
		got, err := Read(buf[off:64])
		if err != nil {
			t.Fatalf("Read err = %v", err)
		}
		if got != len(buf[off:64]) {
			t.Fatalf("Read len = %d", got)
		}
	}
}

func TestRead_NonZeroOutput(t *testing.T) {
	buf := make([]byte, 256)
	_, _ = Read(buf)
	allZero := true
	for _, b := range buf {
		if b != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Fatal("Read produced all-zero output")
	}
}
