package array

import (
	"sync"
	"sync/atomic"
	"unsafe"
)

const mutexLocked = 1 << iota

// mutex: 支持 TryLock
type mutex struct {
	sync.Mutex
}

// TryLock: 尝试获取锁
func (m *mutex) TryLock() bool {
	return atomic.CompareAndSwapInt32((*int32)(unsafe.Pointer(&m.Mutex)), 0, mutexLocked)
}
