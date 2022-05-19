package syncx

import (
	"github.com/songzhibin97/gkit/internal/runtimex"
	"runtime"
	"sync"
	"unsafe"

	"golang.org/x/sys/cpu"
)

const (
	cacheLineSize = unsafe.Sizeof(cpu.CacheLinePad{})
)

var (
	shardsLen int
)

// RWMutex is a p-shard mutex, which has better performance when there's much more read than write.
type RWMutex []rwMutexShard

type rwMutexShard struct {
	sync.RWMutex
	_pad [cacheLineSize - unsafe.Sizeof(sync.RWMutex{})]byte
}

func init() {
	shardsLen = runtime.GOMAXPROCS(0)
}

// NewRWMutex creates a new RWMutex.
func NewRWMutex() RWMutex {
	return make([]rwMutexShard, shardsLen)
}

func (m RWMutex) Lock() {
	for shard := range m {
		m[shard].Lock()
	}
}

func (m RWMutex) Unlock() {
	for shard := range m {
		m[shard].Unlock()
	}
}

func (m RWMutex) RLocker() sync.Locker {
	return m[runtimex.Pid()%shardsLen].RWMutex.RLocker()
}
