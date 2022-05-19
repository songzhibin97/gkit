//go:build !race
// +build !race

package syncx

import (
	_ "sync"
	_ "unsafe"
)

//go:noescape
//go:linkname runtime_registerPoolCleanup sync.runtime_registerPoolCleanup
func runtime_registerPoolCleanup(cleanup func())

//go:noescape
//go:linkname runtime_poolCleanup sync.poolCleanup
func runtime_poolCleanup()
