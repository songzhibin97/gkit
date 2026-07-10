package xxhash3

import "testing"

func TestIssue81CrossArchitectureVectors(t *testing.T) {
	tests := []struct {
		input   string
		hash    uint64
		hash128 [2]uint64
	}{
		{"gkit", 0xba41e5013428a24b, [2]uint64{0xf3405599cacaf09d, 0xc3ed8af5879fb9ef}},
		{"xxhash3-cross-architecture-vector-129-bytes-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", 0xbc582282eda4a279, [2]uint64{0xdd038798dae3fb8d, 0x40a6623525fc377c}},
	}
	for _, tt := range tests {
		input := []byte(tt.input)
		if got := Hash(input); got != tt.hash {
			t.Errorf("Hash(%q) = %#x, want %#x", input, got, tt.hash)
		}
		if got := Hash128(input); got != tt.hash128 {
			t.Errorf("Hash128(%q) = %#x, want %#x", input, got, tt.hash128)
		}
	}
}
