package mutex

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

const mutexLocked = 1 << iota

// Mutex 支持 TryLock
type Mutex struct {
	sync.Mutex
}

// TryLock 尝试获取锁
func (m *Mutex) TryLock() bool {
	return atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&m.Mutex)), 0, mutexLocked)
}
