package buffer

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// AutoExpand 自动展开标识
	AutoExpand = -1

	// ResetOffMark 重叠标记
	ResetOffMark = -1

	// MinRead MaxRead 最大最小读取
	MinRead = 1 << 9
	MaxRead = 1 << 17

	// DefaultSize 默认大小
	DefaultSize = 1 << 4

	// MaxBufferLength 最大缓冲大小
	MaxBufferLength = 1 << 20

	// MaxThreshold 最大阈值
	MaxThreshold = 1 << 22
)

var (
	ErrEOF               = errors.New("EOF")
	ErrTooLarge          = errors.New("io buffer: too large")
	ErrNegativeCount     = errors.New("io buffer: negative count")
	ErrInvalidWriteCount = errors.New("io buffer: invalid write count")
	ErrClosedPipeWrite   = errors.New("write on closed buffer")
	ErrDuplicate         = errors.New("PutIoPool duplicate")
)

// ConnReadTimeout 连接超时时间
var ConnReadTimeout = 15 * time.Second

// nullByte: 空的 []byte
var nullByte []byte

// pipe 管道
type pipe struct {
	// IoBuffer: 继承 IoBuffer 接口
	IoBuffer

	// mu 互斥锁
	mu sync.Mutex

	// c 通知
	c sync.Cond

	// 错误对象
	err error
}

// Len 返回内部 IoBuffer.Len
func (p *pipe) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.IoBuffer == nil {
		return 0
	}
	return p.IoBuffer.Len()
}

// Read 等待数据可用,将缓冲区内容复制到buffer中
func (p *pipe) Read(buffer []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.checkCond()
	for {
		// 如果 IoBuffer != nil && 缓冲区有数据 将其拷贝
		if p.IoBuffer != nil && p.IoBuffer.Len() > 0 {
			return p.IoBuffer.Read(buffer)
		}
		// 快速失败,不再继续
		if p.err != nil {
			return 0, p.err
		}
		// 等待写入,写入完成后会触发 Signal 唤醒读
		p.c.Wait()
	}
}

// Write 将 buffer 的字节写入到缓冲区并唤醒读取
// 写入的数据超过缓冲区的容量是错误的
func (p *pipe) Write(buffer []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.checkCond()
	// 唤醒一个读取
	defer p.c.Signal()
	if p.err != nil {
		return 0, ErrClosedPipeWrite
	}
	return len(buffer), p.IoBuffer.Append(buffer)
}

// CloseWithError 使下一次读取(如果需要,唤醒当前阻止的读取)在所有数据都已经完成后返回提供错误
func (p *pipe) CloseWithError(err error) {
	if err == nil {
		err = io.EOF
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.checkCond()
	p.err = err
	defer p.c.Signal()
}

// checkCond 检查 Cond 是否初始化, 否则赋值锁
func (p *pipe) checkCond() {
	if p.c.L == nil {
		p.c.L = &p.mu
	}
}

// NewPipe 初始化 pipe IoBuffer
func NewPipe(cap int) IoBuffer {
	return &pipe{
		IoBuffer: newIoBuffer(cap),
	}
}

// ioBuffer 实现 IoBuffer 接口
type ioBuffer struct {
	// contents: buffer[off : len(buffer)]
	buffer []byte
	// 从 &buffer[off] 读取,
	// 从 &buffer[len(buffer)] 写入
	// off 偏移量
	off     int
	offMark int
	count   int32
	eof     bool

	b *[]byte
}

// Read 从缓冲区读取拷贝到 p
func (i *ioBuffer) Read(p []byte) (n int, err error) {
	if i.off > 0 && i.off >= len(i.buffer) {
		// off已经漂移到buffer末尾或越界
		i.Reset()
		if len(p) == 0 {
			return
		}
		return 0, io.EOF
	}
	// 将buffer后续的拷贝到 p上 返回 n
	// off 继续偏移
	n = copy(p, i.buffer[i.off:])
	i.off += n
	return
}

// ReadOnce 从传入参数 r io.Reader 读取一次
func (i *ioBuffer) ReadOnce(r io.Reader) (n int64, err error) {
	if i.off > 0 && i.off >= len(i.buffer) {
		// off已经漂移到buffer末尾或越界
		i.Reset()
	}
	if i.off >= (cap(i.buffer) - len(i.buffer)) {
		// cap - len 等于可用空间
		// off > 可用空间
		i.copy(0)
	}

	// 可用的最大缓冲区避免内存泄漏
	if i.off == len(i.buffer) && cap(i.buffer) > MaxBufferLength {
		i.Free()
		i.Alloc(MaxRead)
	}
	l := cap(i.buffer) - len(i.buffer)

	var m int
	m, err = r.Read(i.buffer[len(i.buffer):cap(i.buffer)])

	i.buffer = i.buffer[0 : len(i.buffer)+m]
	n = int64(m)

	// 任何地方没有足够的空间,需要分配
	if l == m {
		i.copy(AutoExpand)
	}
	return n, err
}

// ReadFrom 从传入参数 r io.Reader 循环读取
func (i *ioBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	if i.off > 0 && i.off >= len(i.buffer) {
		// off已经漂移到buffer末尾或越界
		i.Reset()
	}

	for {
		if free := cap(i.buffer) - len(i.buffer); free < MinRead {
			if i.off+free < MinRead {
				// 没有足够的空间,需要扩容
				i.copy(MinRead)
			} else {
				i.copy(0)
			}
		}

		m, e := r.Read(i.buffer[len(i.buffer):cap(i.buffer)])

		i.buffer = i.buffer[0 : len(i.buffer)+m]
		n += int64(m)

		if err == io.EOF || m == 0 {
			break
		}

		if e != nil {
			return n, e
		}
	}

	return
}

// Grow 扩张
func (i *ioBuffer) Grow(n int) error {
	if _, ok := i.tryGrowByRelies(n); !ok {
		i.grow(n)
	}
	return nil
}

// Cup 将 offset 的内容切分出来变成一个新的 IoBuffer
func (i *ioBuffer) Cup(offset int) IoBuffer {
	if i.off+offset > len(i.buffer) {
		// 如果读取的位置+偏移量大于现在写的位置,无效操作
		return nil
	}
	buf := make([]byte, offset)
	copy(buf, i.buffer[i.off:i.off+offset])
	i.off += offset
	i.offMark = ResetOffMark
	return &ioBuffer{buffer: buf, off: 0}
}

// Write 将 p 写入到缓冲区
func (i *ioBuffer) Write(p []byte) (n int, err error) {
	m, ok := i.tryGrowByRelies(len(p))
	if !ok {
		m = i.grow(len(p))
	}
	return copy(i.buffer[m:], p), nil
}

// WriteString 将 s 写入到缓冲区
func (i *ioBuffer) WriteString(s string) (n int, err error) {
	m, ok := i.tryGrowByRelies(len(s))
	if !ok {
		m = i.grow(len(s))
	}
	return copy(i.buffer[m:], s), nil
}

// WriteByte 将 byte 写入到缓冲区
func (i *ioBuffer) WriteByte(p byte) error {
	m, ok := i.tryGrowByRelies(1)
	if !ok {
		m = i.grow(1)
	}
	i.buffer[m] = p
	return nil
}

// WriteUint16 将 uint16 写入缓冲区
func (i *ioBuffer) WriteUint16(p uint16) error {
	m, ok := i.tryGrowByRelies(2)
	if !ok {
		m = i.grow(2)
	}
	// 大端写入
	binary.BigEndian.PutUint16(i.buffer[m:], p)
	return nil
}

// WriteUint32 将 uint32 写入缓冲区
func (i *ioBuffer) WriteUint32(p uint32) error {
	m, ok := i.tryGrowByRelies(4)
	if !ok {
		m = i.grow(4)
	}
	// 大端写入
	binary.BigEndian.PutUint32(i.buffer[m:], p)
	return nil
}

// WriteUint64 将 uint64 写入缓冲区
func (i *ioBuffer) WriteUint64(p uint64) error {
	m, ok := i.tryGrowByRelies(8)
	if !ok {
		m = i.grow(8)
	}
	// 大端写入
	binary.BigEndian.PutUint64(i.buffer[m:], p)
	return nil
}

// WriteTo 写入传入参数  w io.Writer
func (i *ioBuffer) WriteTo(w io.Writer) (n int64, err error) {
	for i.off < len(i.buffer) {
		nBytes := i.Len()
		m, e := w.Write(i.buffer[i.off:])
		if m > nBytes {
			panic(ErrInvalidWriteCount)
		}

		i.off += m
		n += int64(m)

		if e != nil {
			return n, e
		}

		if m == 0 || m == nBytes {
			return n, nil
		}
	}
	return
}

// Peek 从缓冲区读取n个字节,不会消耗缓冲区,如果超过或不合法返回nil
func (i *ioBuffer) Peek(n int) []byte {
	if len(i.buffer)-i.off < n {
		return nil
	}
	return i.buffer[i.off : i.off+n]
}

// Mark 设置标记
func (i *ioBuffer) Mark() {
	i.offMark = i.off
}

// Bytes 返回缓冲区所有字节,不会消耗缓冲区
func (i *ioBuffer) Bytes() []byte {
	return i.buffer[i.off:]
}

// Drain 排出缓冲区 offset 长度
func (i *ioBuffer) Drain(offset int) {
	if i.off+offset > len(i.buffer) {
		return
	}
	i.off += offset
	i.offMark = ResetOffMark
}

// Len 返回缓冲区未读的字节
func (i *ioBuffer) Len() int {
	return len(i.buffer) - i.off
}

// Cap 返回底层切片容量
func (i *ioBuffer) Cap() int {
	return cap(i.buffer)
}

// Reset 将缓冲区置空
func (i *ioBuffer) Reset() {
	i.buffer = i.buffer[:0]
	i.off = 0
	i.offMark = ResetOffMark
	i.eof = false
}

// Restore 从标记处恢复偏移量
func (i *ioBuffer) Restore() {
	if i.offMark != ResetOffMark {
		i.off = i.offMark
		i.offMark = ResetOffMark
	}
}

// Clone 克隆复制 IoBuffer 结构
func (i *ioBuffer) Clone() IoBuffer {
	buf := GetIoPool(i.Len())
	_, _ = buf.Write(i.Bytes())
	buf.SetEOF(i.EOF())
	return buf
}

// String 返回缓冲区未读的部分内容,作为字符串,如果底层buffer 为nil 返回 "<nil>"
func (i *ioBuffer) String() string {
	return string(i.buffer[i.off:])
}

// Alloc 从 bytePool 获取到 buffer
func (i *ioBuffer) Alloc(size int) {
	if i.buffer != nil {
		i.Free()
	}
	if size <= 0 {
		size = DefaultSize
	}
	i.b = i.makeSlice(size)
	i.buffer = *i.b
	i.buffer = i.buffer[:0]
}

// Free 释放到 bytePool 中
func (i *ioBuffer) Free() {
	i.Reset()
	i.putSlice()
}

// Count 计数集并返回参考计数
func (i *ioBuffer) Count(count int32) int32 {
	return atomic.AddInt32(&i.count, count)
}

// EOF 是否EOF终止
func (i *ioBuffer) EOF() bool {
	return i.eof
}

// SetEOF 设置eof状态
func (i *ioBuffer) SetEOF(eof bool) {
	i.eof = eof
}

// Append 将 []byte 写入缓冲区
func (i *ioBuffer) Append(data []byte) error {
	if i.off > 0 && i.off >= len(i.buffer) {
		i.Reset()
	}

	dataLen := len(data)

	if free := cap(i.buffer) - len(i.buffer); free < dataLen {
		// 没有足够空间
		if i.off+free > dataLen {
			i.copy(0)
		} else {
			i.copy(dataLen)
		}
	}
	m := copy(i.buffer[len(i.buffer):len(i.buffer)+dataLen], data)
	i.buffer = i.buffer[0 : len(i.buffer)+m]
	return nil
}

// AppendByte 将 byte 写入缓冲区
func (i *ioBuffer) AppendByte(data byte) error {
	return i.Append([]byte{data})
}

func (i *ioBuffer) CloseWithError(error) {}

// putSlice 将slice释放
func (i *ioBuffer) putSlice() {
	if i.b != nil {
		PutBytes(i.b)
		i.b = nil
		i.buffer = nullByte
	}
}

// grow 增长
func (i *ioBuffer) grow(n int) int {
	m := i.Len()

	// 如果缓冲区为空,重置空间
	if m == 0 && i.off != 0 {
		i.Reset()
	}

	// 尝试通过重新切分
	if ret, ok := i.tryGrowByRelies(n); ok {
		return ret
	}

	if m+n <= cap(i.buffer)/2 {
		// 数据顺推,不许要重新分配slice
		// 只需要  m+n <= cap(i.buffer)
		i.copy(0)
	} else {
		// 让容量扩展一倍
		// 不需要将所有的时间用在 copy上
		i.copy(n)
	}
	i.off = 0
	i.buffer = i.buffer[:m+n]
	return m
}

// tryGrowByRelies 尝试通过切分达到增长
// 判断 len(buffer) + n <= cap(buffer) 满足的话, 将buffer扩展到 len(buffer) + n
func (i *ioBuffer) tryGrowByRelies(n int) (int, bool) {
	if l := len(i.buffer); l+n <= cap(i.buffer) {
		i.buffer = i.buffer[:l+n]
		return l, true
	}
	return 0, false
}

// expand 扩缩容
// 如果 expand > 0 cap newBuffer 根据 oldBuffer 计算 并展开
// 如果 expand == 0 仅拷贝,不会扩展
// 如果 expand == AutoExpand cap newBuffer 仅根据 oldBuffer 计算
func (i *ioBuffer) copy(expand int) {
	var (
		newBuf  []byte
		bufferP *[]byte
	)
	if expand > 0 || expand == AutoExpand {
		cap_ := cap(i.buffer)
		// 当buf上限大于 MaxThreshold 时，启动Slow Grow
		if cap_ < 2*MinRead {
			cap_ = 2 * MinRead
		} else if cap_ < MaxThreshold {
			cap_ = 2 * cap_
		} else {
			cap_ = cap_ + cap_/4
		}
		if expand == AutoExpand {
			expand = 0
		}
		bufferP = i.makeSlice(cap_ + expand)
		newBuf = *bufferP
		copy(newBuf, i.buffer[i.off:])
		PutBytes(i.b)
		i.b = bufferP
	} else {
		newBuf = i.buffer
		copy(newBuf, i.buffer[i.off:])
	}
	i.buffer = newBuf[:len(i.buffer)-i.off]
	i.off = 0
}

func (i *ioBuffer) makeSlice(n int) *[]byte {
	return GetBytes(n)
}

// newIoBuffer ioBuffer 初始化 IoBuffer
func newIoBuffer(cap int) IoBuffer {
	buffer := &ioBuffer{
		offMark: ResetOffMark,
		count:   1,
	}
	if cap <= 0 {
		cap = DefaultSize
	}
	buffer.b = GetBytes(cap)
	buffer.buffer = (*buffer.b)[:0]
	return buffer
}

// NewIoBufferString string 生成 IoBuffer
func NewIoBufferString(s string) IoBuffer {
	if s == "" {
		return newIoBuffer(0)
	}
	return &ioBuffer{
		buffer:  []byte(s),
		offMark: ResetOffMark,
		count:   1,
	}
}

// NewIoBufferBytes []byte 生成 IoBuffer
func NewIoBufferBytes(bytes []byte) IoBuffer {
	if bytes == nil {
		return newIoBuffer(0)
	}
	return &ioBuffer{
		buffer:  bytes,
		offMark: ResetOffMark,
		count:   1,
	}
}

// NewIoBufferEOF 生成一个EOF的 IoBuffer
func NewIoBufferEOF() IoBuffer {
	buffer := newIoBuffer(0)
	buffer.SetEOF(true)
	return buffer
}
