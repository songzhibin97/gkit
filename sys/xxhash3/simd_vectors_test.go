package xxhash3

import (
	"runtime"
	"testing"

	"golang.org/x/sys/cpu"
)

func TestSIMDBoundaryVectors(t *testing.T) {
	// xxHash v0.8.3 C reference, with input[i] = byte(i*17+3):
	// https://github.com/Cyan4973/xxHash/blob/e626a72bc2321cd320e953a0ccf1584cad60f363/xxhash.h
	vectors := []struct {
		n       int
		hash    uint64
		hash128 [2]uint64 // high, low
	}{
		{241, 0xcccc61a5efbadc76, [2]uint64{0x9dd034d5fbc3012b, 0xcccc61a5efbadc76}},
		{1023, 0x036c568b42ad1ce5, [2]uint64{0x8825c09ab91b1372, 0x036c568b42ad1ce5}},
		{1024, 0x0f98cd305cb2e45f, [2]uint64{0x3aa2d7166245e946, 0x0f98cd305cb2e45f}},
		{1025, 0x6ec54dffae085c90, [2]uint64{0xe49ced3861e402b0, 0x6ec54dffae085c90}},
		{2049, 0xa1d97adb92628b10, [2]uint64{0xbe021ee278e6350d, 0xa1d97adb92628b10}},
	}
	oldAVX2, oldSSE2 := avx2, sse2
	t.Cleanup(func() { avx2, sse2 = oldAVX2, oldSSE2 })
	t.Logf("architecture=%s SSE2=%v AVX2=%v", runtime.GOARCH, cpu.X86.HasSSE2, cpu.X86.HasAVX2)
	for _, backend := range []struct {
		name                string
		avx, sse, supported bool
	}{
		{"scalar", false, false, true},
		{"SSE2", false, true, runtime.GOARCH == "amd64" && cpu.X86.HasSSE2},
		{"AVX2", true, false, runtime.GOARCH == "amd64" && cpu.X86.HasAVX2},
	} {
		t.Run(backend.name, func(t *testing.T) {
			if !backend.supported {
				t.Skip("CPU feature unavailable")
			}
			avx2, sse2 = backend.avx, backend.sse
			for _, vector := range vectors {
				storage := make([]byte, vector.n+1)
				input := storage[1:] // Exercise unaligned SIMD loads too.
				for i := range input {
					input[i] = byte(i*17 + 3)
				}
				if got := Hash(input); got != vector.hash {
					t.Errorf("Hash len=%d: got %#x, want %#x", vector.n, got, vector.hash)
				}
				if got := Hash128(input); got != vector.hash128 {
					t.Errorf("Hash128 len=%d: got %#x, want %#x", vector.n, got, vector.hash128)
				}
			}
		})
	}
}
