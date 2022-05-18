package runtimex

import (
	_ "unsafe"
)

//go:noescape
//go:linkname runtime_procPin runtime.procPin
func runtime_procPin() int

//go:noescape
//go:linkname runtime_procUnpin runtime.procUnpin
func runtime_procUnpin()

// Pin pins current p, return pid.
// DO NOT USE if you don't know what this is.
func Pin() int {
	return runtime_procPin()
}

// Unpin unpins current p.
func Unpin() {
	runtime_procUnpin()
}

// Pid returns the id of current p.
func Pid() (id int) {
	id = runtime_procPin()
	runtime_procUnpin()
	return
}
