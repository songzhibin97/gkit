package syncx

import (
	"sync"
	"testing"
	"unsafe"
)

func TestRWMutexShardPadding(t *testing.T) {
	const alignment = uintptr(128)

	shardSize := unsafe.Sizeof(rwMutexShard{})
	if shardSize < unsafe.Sizeof(sync.RWMutex{}) {
		t.Fatalf("shard size %d is smaller than sync.RWMutex", shardSize)
	}
	if shardSize%alignment != 0 {
		t.Fatalf("shard size %d is not a multiple of %d", shardSize, alignment)
	}
}
