package buffer

// Byte 复用
func ExampleGetBytes() {
	// size 2^6 - 2^18
	// 返回向上取整的 2的整数倍 cap, len == size
	// 其他特殊的或者在运行期间扩容的 将会被清空
	slice := GetBytes(1024)
	_ = slice
}

func ExamplePutBytes() {
	slice := make([]byte, 1024)
	// 将slice回收
	PutBytes(&slice)
}

// IOByte 复用

func ExampleGetIoPool() {
	// 创建一个缓冲区为 cap大小的 io对象
	io := GetIoPool(1024)
	_ = io
}

func ExamplePutIoPool() {
	mockIoPool := newIoBuffer(1024)
	err := PutIoPool(mockIoPool)
	if err != nil {
		// 如果一个对象已经被回收了,再次引用被回收的对象会触发错误
	}
}
