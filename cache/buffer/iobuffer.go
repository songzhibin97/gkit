package buffer

import (
	"errors"
	"io"
	"sync"
	"time"
)

const (
	// AutoExpand: 自动展开标识
	AutoExpand = -1

	// ResetOffMark: 重叠标记
	ResetOffMark = -1

	// MinRead MaxRead: 最大最小读取
	MinRead = 1 << 9
	MaxRead = 1 << 17

	// DefaultSize: 默认大小
	DefaultSize = 1 << 4

	// MaxBufferLength: 最大缓冲大小
	MaxBufferLength = 1 << 20

	// MaxThreshold: 最大阈值
	MaxThreshold = 1 << 22
)

var (
	EOF                  = errors.New("EOF")
	ErrTooLarge          = errors.New("io buffer: too large")
	ErrNegativeCount     = errors.New("io buffer: negative count")
	ErrInvalidWriteCount = errors.New("io buffer: invalid write count")
	ErrClosedPipeWrite   = errors.New("write on closed buffer")
)

// ConnReadTimeout: 连接超时时间
var ConnReadTimeout = 15 * time.Second

// nullByte: 空的 []byte
var nullByte []byte

// pipe: 管道
type pipe struct {
	// IoBuffer: 继承 IoBuffer 接口
	IoBuffer

	// mu: 互斥锁
	mu sync.Mutex

	// c: 通知
	c sync.Cond

	// 错误对象
	err error
}

// Len: 返回内部 IoBuffer.Len
func (p *pipe) Len() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.IoBuffer == nil {
		return 0
	}
	return p.IoBuffer.Len()
}

// Read: 等待数据可用,将缓冲区内容复制到buffer中
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

// Write: 将 buffer 的字节写入到缓冲区并唤醒读取
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

// CloseWithError: 使下一次读取(如果需要,唤醒当前阻止的读取)在所有数据都已经完成后返回提供错误
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

// checkCond: 检查 Cond 是否初始化, 否则赋值锁
func (p *pipe) checkCond() {
	if p.c.L == nil {
		p.c.L = &p.mu
	}
}

// NewPipe: 初始化 pipe IoBuffer
func NewPipe(cap int) IoBuffer {
	return &pipe{
		IoBuffer: newIoBuffer(cap),
	}
}

// ioBuffer 实现 IoBuffer 接口
type ioBuffer struct {
	// contents: buffer[off : len(buffer)]
	buffer []byte
	// 从 &buf[off] 读取,
	// 从 &buffer[len(buffer)] 写入
	off     int
	offMark int
	count   int32
	eof     bool

	b *[]byte
}

func (i ioBuffer) Read(p []byte) (n int, err error) {
	panic("implement me")
}

func (i ioBuffer) ReadOnce(r io.Reader) (n int64, err error) {
	panic("implement me")
}

func (i ioBuffer) ReadFrom(r io.Reader) (n int64, err error) {
	panic("implement me")
}

func (i ioBuffer) Grow(n int) error {
	panic("implement me")
}

func (i ioBuffer) Write(p []byte) (n int, err error) {
	panic("implement me")
}

func (i ioBuffer) WriteString(s string) (n int, err error) {
	panic("implement me")
}

func (i ioBuffer) WriteByte(p byte) error {
	panic("implement me")
}

func (i ioBuffer) WriteUint16(p uint16) error {
	panic("implement me")
}

func (i ioBuffer) WriteUint32(p uint32) error {
	panic("implement me")
}

func (i ioBuffer) WriteUint64(p uint64) error {
	panic("implement me")
}

func (i ioBuffer) WriteTo(w io.Writer) (n int64, err error) {
	panic("implement me")
}

func (i ioBuffer) Peek(n int) []byte {
	panic("implement me")
}

func (i ioBuffer) Bytes() []byte {
	panic("implement me")
}

func (i ioBuffer) Drain(offset int) {
	panic("implement me")
}

func (i ioBuffer) Len() int {
	panic("implement me")
}

func (i ioBuffer) Cap() int {
	panic("implement me")
}

func (i ioBuffer) Reset() {
	panic("implement me")
}

func (i ioBuffer) Clone() IoBuffer {
	panic("implement me")
}

func (i ioBuffer) String() string {
	panic("implement me")
}

func (i ioBuffer) Alloc(i2 int) {
	panic("implement me")
}

func (i ioBuffer) Free() {
	panic("implement me")
}

func (i ioBuffer) Count(i2 int32) int32 {
	panic("implement me")
}

func (i ioBuffer) EOF() bool {
	panic("implement me")
}

func (i ioBuffer) SetEOF(eof bool) {
	panic("implement me")
}

func (i ioBuffer) Append(data []byte) error {
	panic("implement me")
}

func (i ioBuffer) CloseWithError(err error) {
	panic("implement me")
}

// newIoBuffer: ioBuffer 初始化 IoBuffer
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

// NewIoBufferString: string 生成 IoBuffer
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

// NewIoBufferBytes: []byte 生成 IoBuffer
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

// NewIoBufferEOF: 生成一个EOF的 IoBuffer
func NewIoBufferEOF() IoBuffer {
	buffer := newIoBuffer(0)
	buffer.SetEOF(true)
	return buffer
}
