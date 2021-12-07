package buffer

import "sync"

const (
	minShift = 6
	maxShift = 18

	// errSlot: 如果没有找到对应的 slot 返回 -1
	errSlot = -1
)

var localBytePool = newBytePool()

// byteSlot 槽区
type byteSlot struct {
	// defaultSize 默认 slot 存储 []byte 大小
	defaultSize int

	// pool 实际上 pool池
	pool sync.Pool
}

// bytePool []byte pool
type bytePool struct {
	minShift int
	minSize  int
	maxSize  int

	// 维护 byteSlot
	pool []*byteSlot
}

// slot 根据 size 获取到 对应的 byteSlot 下标
func (b *bytePool) slot(size int) int {
	// 超过阈值
	if size > b.maxSize {
		return errSlot
	}
	slot := 0
	shift := 0
	if size > b.minSize {
		size--
		for size > 0 {
			size = size >> 1
			shift++
		}
		slot = shift - b.minShift
	}
	return slot
}

// get 根据 size 从 bytePool 中获取到 *[]byte
func (b *bytePool) get(size int) *[]byte {
	slot := b.slot(size)
	if slot == errSlot {
		// 如果需要的 []byte 大于 设置的 pool 返回 errSlot
		// 触发 errSlot 会手动创建一个 []byte 返回
		ret := newBytes(size)
		return &ret
	}
	v := b.pool[slot].pool.Get()
	if v == nil {
		// 如果返回的 v == nil
		// 手动创建...
		ret := newBytes(b.pool[slot].defaultSize)
		ret = ret[0:size]
		return &ret
	}
	ret := v.(*[]byte)
	*ret = (*ret)[0:size]
	return ret
}

// put 将 *[]byte 归还给 bytePool
func (b *bytePool) put(buf *[]byte) {
	if buf == nil {
		return
	}
	// 获取到 size 大小
	size := cap(*buf)
	slot := b.slot(size)
	if slot == errSlot || size != b.pool[slot].defaultSize {
		// 说明是特殊创建的 []byte 直接释放
		// != defaultSize 有可能在执行过程中扩容了
		return
	}
	// 归还
	b.pool[slot].pool.Put(buf)
}

// newBytes 手动创建 []byte
func newBytes(size int) []byte {
	return make([]byte, size)
}

// newBytePool 实例化
func newBytePool() *bytePool {
	b := &bytePool{
		minShift: minShift,
		minSize:  1 << minShift,
		maxSize:  1 << maxShift,
	}
	for i := (uint)(0); i < maxShift-minShift; i++ {
		slot := &byteSlot{defaultSize: 1 << (i + minShift)}
		b.pool = append(b.pool, slot)
	}
	return b
}

// BytePoolContainer 暴露给外部使用的容器对象
type BytePoolContainer struct {
	bytes []*[]byte
	*bytePool
}

// Reset 将 bytes 中缓存的buffer全部归还给 pool中
func (B *BytePoolContainer) Reset() {
	for _, buf := range B.bytes {
		B.put(buf)
	}
	B.bytes = B.bytes[:0]
}

func (B *BytePoolContainer) Get(size int) *[]byte {
	buf := B.get(size)
	B.bytes = append(B.bytes, buf)
	return buf
}

// NewBytePoolContainer 实例化外部容器
func NewBytePoolContainer() *BytePoolContainer {
	return &BytePoolContainer{
		bytePool: localBytePool,
	}
}

// GetBytes 提供外部接口 获取 size 大小的 buffer
func GetBytes(size int) *[]byte {
	return localBytePool.get(size)
}

// PutBytes 提供外部接口 将buffer 放回 pool中
func PutBytes(buf *[]byte) {
	localBytePool.put(buf)
}
