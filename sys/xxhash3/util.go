// Package xxhash3 implements https://github.com/Cyan4973/xxHash/blob/dev/xxhash.h
package xxhash3

import (
	"math/bits"
	"unsafe"

	"golang.org/x/sys/cpu"
)

var (
	avx2        = cpu.X86.HasAVX2
	sse2        = cpu.X86.HasSSE2
	hashfunc    = [2]func(unsafe.Pointer, int) uint64{xxh3HashSmall, xxh3HashLarge}
	hashfunc128 = [2]func(unsafe.Pointer, int) [2]uint64{xxh3HashSmall128, xxh3HashLarge128}
)

type funcUnsafe int

const (
	hashSmall funcUnsafe = iota
	hashLarge
)

func mix(a, b uint64) uint64 {
	hi, lo := bits.Mul64(a, b)
	return hi ^ lo
}
func xxh3RRMXMX(h64 uint64, length uint64) uint64 {
	h64 ^= bits.RotateLeft64(h64, 49) ^ bits.RotateLeft64(h64, 24)
	h64 *= 0x9fb21c651e98df25
	h64 ^= (h64 >> 35) + length
	h64 *= 0x9fb21c651e98df25
	h64 ^= (h64 >> 28)
	return h64
}

func xxh64Avalanche(h64 uint64) uint64 {
	h64 *= prime64_2
	h64 ^= h64 >> 29
	h64 *= prime64_3
	h64 ^= h64 >> 32
	return h64
}

func xxh3Avalanche(x uint64) uint64 {
	x ^= x >> 37
	x *= 0x165667919e3779f9
	x ^= x >> 32
	return x
}
