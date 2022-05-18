// Package runtimex implements https://github.com/bytedance/gopkg
package runtimex

import (
	_ "unsafe" // for linkname
)

//go:linkname Fastrand runtime.fastrand
func Fastrand() uint32
