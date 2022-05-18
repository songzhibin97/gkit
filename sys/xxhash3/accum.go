//go:build !amd64
// +build !amd64

package xxhash3

import "unsafe"

func accum(xacc *[8]uint64, xinput, xsecret unsafe.Pointer, l uintptr) {
	accumScalar(xacc, xinput, xsecret, l)
}
