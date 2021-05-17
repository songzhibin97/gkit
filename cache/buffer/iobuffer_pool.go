package buffer

import "sync"

// localIOPool
var localIOPool IoPool

// IoPool 存储 IoBuffer Pool
type IoPool struct {
	// IoBuffer
	pool sync.Pool
}

// get 从pool中 获取一个 IoBuffer
func (i *IoPool) get(size int) IoBuffer {
	v := i.pool.Get()
	if v == nil {
		return newIoBuffer(size)
	} else {
		buf := v.(IoBuffer)
		buf.Alloc(size)
		buf.Count(1)
		return buf
	}
}

// put 向pool中回填一个 IoBuffer
func (i *IoPool) put(buf IoBuffer) {
	buf.Free()
	i.pool.Put(buf)
}

// GetIoPool 从pool中 获取一个 IoBuffer
func GetIoPool(size int) IoBuffer {
	return localIOPool.get(size)
}

// PutIoPool 向pool中回填一个 IoBuffer
func PutIoPool(buf IoBuffer) error {
	count := buf.Count(-1)
	if count > 0 {
		// 还有其他引用
		return nil
	} else if count < 0 {
		return ErrDuplicate
	}
	if p, _ := buf.(*pipe); p != nil {
		buf = p.IoBuffer
	}
	localIOPool.put(buf)
	return nil
}

// NewIoBuffer GetIoPool 别名
func NewIoBuffer(size int) IoBuffer {
	return GetIoPool(size)
}
