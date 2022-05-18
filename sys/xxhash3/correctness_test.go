package xxhash3

import (
	"github.com/songzhibin97/gkit/sys/xxhash3/internal/xxh3_raw"
	"math/rand"
	"testing"

	"golang.org/x/sys/cpu"
)

var dat []byte

const capacity = 1<<25 + 100000

func init() {
	dat = make([]byte, capacity)
	for i := 0; i < capacity; i++ {
		dat[i] = byte(rand.Int31())
	}
}

func TestLen0_16(t *testing.T) {
	for i := 0; i <= 16; i++ {
		input := dat[:i]
		res1 := xxh3_raw.Hash(input)
		res2 := Hash(input)

		if res1 != res2 {
			t.Fatal("Wrong answer")
		}
	}
}

func TestLen17_128(t *testing.T) {
	for i := 17; i <= 128; i++ {
		input := dat[:i]
		res1 := xxh3_raw.Hash(input)
		res2 := Hash(input)

		if res1 != res2 {
			t.Fatal("Wrong answer")
		}
	}
}

func TestLen129_240(t *testing.T) {

	for i := 129; i <= 240; i++ {
		input := dat[:i]
		res1 := xxh3_raw.Hash(input)
		res2 := Hash(input)

		if res1 != res2 {
			t.Fatal("Wrong answer")
		}
	}
}

func TestLen240_1024(t *testing.T) {
	avx2, sse2 = false, false

	for i := 240; i <= 1024; i++ {
		input := dat[:i]
		res1 := xxh3_raw.Hash(input)
		res2 := Hash(input)

		if res1 != res2 {
			t.Fatal("Wrong answer")
		}
	}
}

func TestLen1025_1048576_scalar(t *testing.T) {
	avx2, sse2 = false, false
	for i := 1025; i < 1048576; i = i << 1 {
		input := dat[:i]
		res1 := xxh3_raw.Hash(input)
		res2 := Hash(input)

		if res1 != res2 {
			t.Fatal("Wrong answer", i)
		}
	}
}

func TestLen1024_1048576_AVX2(t *testing.T) {
	avx2, sse2 = cpu.X86.HasAVX2, false
	for i := 1024; i < 1048576; i = i << 1 {
		input := dat[:i]
		res1 := xxh3_raw.Hash(input)
		res2 := Hash(input)

		if res1 != res2 {
			t.Fatal("Wrong answer", i)
		}
	}
}

func TestLen1024_1048576_SSE2(t *testing.T) {
	avx2, sse2 = false, cpu.X86.HasSSE2
	for i := 1024; i < 1048576; i = i << 1 {
		input := dat[:i]
		res1 := xxh3_raw.Hash(input)
		res2 := Hash(input)

		if res1 != res2 {
			t.Fatal("Wrong answer")
		}
	}
}

func TestLen128_0_16(t *testing.T) {
	for i := 0; i <= 16; i++ {
		input := dat[:i]
		res1 := xxh3_raw.Hash128(input)
		res2 := Hash128(input)

		if res1 != res2 {
			t.Fatal("Wrong answer")
		}
	}
}

func TestLen128_17_128(t *testing.T) {
	for i := 17; i <= 128; i++ {
		input := dat[:i]
		res1 := xxh3_raw.Hash128(input)
		res2 := Hash128(input)

		if res1 != res2 {
			t.Fatal("Wrong answer")
		}
	}
}

func TestLen128_129_240(t *testing.T) {

	for i := 129; i <= 240; i++ {
		input := dat[:i]
		res1 := xxh3_raw.Hash128(input)
		res2 := Hash128(input)

		if res1 != res2 {
			t.Fatal("Wrong answer")
		}
	}
}

func TestLen128_240_1024(t *testing.T) {
	avx2, sse2 = false, false

	for i := 240; i <= 1024; i++ {
		input := dat[:i]
		res1 := xxh3_raw.Hash128(input)
		res2 := Hash128(input)

		if res1 != res2 {
			t.Fatal("Wrong answer")
		}
	}
}

func TestLen128_1025_1048576_scalar(t *testing.T) {
	avx2, sse2 = false, false
	for i := 1025; i < 1048576; i = i << 1 {
		input := dat[:i]
		res1 := xxh3_raw.Hash128(input)
		res2 := Hash128(input)

		if res1 != res2 {
			t.Fatal("Wrong answer", i)
		}
	}
}

func TestLen128_1024_1048576_AVX2(t *testing.T) {
	avx2, sse2 = cpu.X86.HasAVX2, false

	for i := 1024; i < 1048576; i = i << 1 {
		input := dat[:i]
		res1 := xxh3_raw.Hash128(input)
		res2 := Hash128(input)

		if res1 != res2 {
			t.Fatal("Wrong answer", i)
		}
	}
}

func TestLen128_1024_1048576_SSE2(t *testing.T) {
	avx2, sse2 = false, cpu.X86.HasSSE2

	for i := 1024; i < 1048576; i = i << 1 {
		input := dat[:i]
		res1 := xxh3_raw.Hash128(input)
		res2 := Hash128(input)

		if res1 != res2 {
			t.Fatal("Wrong answer")
		}
	}
}
