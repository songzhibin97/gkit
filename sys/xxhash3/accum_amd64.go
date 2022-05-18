package xxhash3

import "unsafe"

func accumAVX2(acc *[8]uint64, xinput, xsecret unsafe.Pointer, len uintptr)
func accumSSE2(acc *[8]uint64, xinput, xsecret unsafe.Pointer, len uintptr)

func accum(xacc *[8]uint64, xinput, xsecret unsafe.Pointer, l uintptr) {
	if avx2 {
		accumAVX2(xacc, xinput, xsecret, l)
	} else if sse2 {
		accumSSE2(xacc, xinput, xsecret, l)
	} else {
		accumScalar(xacc, xinput, xsecret, l)
	}
}
