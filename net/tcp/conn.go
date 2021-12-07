package tcp

import (
	"bufio"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"time"

	"github.com/songzhibin97/gkit/cache/buffer"
)

var defaultRetry Retry

// Conn 封装原始 net.conn 对象
type Conn struct {
	// net.conn: 原始的conn对象
	net.Conn

	// reader: 用于读取conn缓冲区
	reader *bufio.Reader

	// sendTimeout: 发送超时时间
	sendTimeout time.Time

	// recvTimeout: 接受超时时间
	recvTimeout time.Time

	// recvBufferInterval: 读取缓存间隔时间
	recvBufferInterval time.Duration
}

// Retry 重试配置
type Retry struct {
	// Count: 重试次数,每重试一次就会 -1, 如果==0默认不重试
	Count uint

	// Interval: 重试间隔
	Interval time.Duration
}

// Send 发送数据至对端,有重试机制
func (c *Conn) Send(data []byte, retry *Retry) error {
	if retry == nil {
		retry = &defaultRetry
	}
	for {
		_, err := c.Write(data)
		switch {
		case err != nil && errors.Is(err, io.EOF):
			// EOF 处理
			return nil
		case err != nil && retry.Count > 0:
			// 触发重试
			retry.Count--
			if retry.Interval == 0 {
				retry.Interval = DefaultRetryInterval
			}
			time.Sleep(retry.Interval)
		default:
			return err
		}
	}
}

// Recv 接受数据
// length == 0 从 Conn一次读取立即返回
// length < 0 从 Conn 接收所有数据，并将其返回，直到没有数据
// length > 0 从 Conn 接收到对应的数据返回
func (c *Conn) Recv(length int, retry *Retry) ([]byte, error) {
	if retry == nil {
		retry = &defaultRetry
	}
	var (
		// err: error
		err error

		// size: 返回一次读取的大小
		size int

		// index: 目前指向的索引的位置
		index int

		// bf: 读取后的缓冲区
		bf []byte

		// flag: 判断是否循环读
		flag bool
	)
	if length > 0 {
		// 读取指定的长度
		bf = *buffer.GetBytes(length)
	} else {
		// 需要 eof 返回
		bf = *buffer.GetBytes(DefaultReadBuffer)
	}
cycle:
	for {
		if length < 0 && index > 0 {
			// length < 0 要接受所有的数据,直至EOF
			flag = true
			if err = c.SetReadDeadline(time.Now().Add(c.recvBufferInterval)); err != nil {
				return nil, err
			}
		}
		size, err = c.reader.Read(bf[index:])
		if size > 0 {
			index += size
			if length > 0 {
				if index == length {
					break cycle
				}
			} else {
				if index >= DefaultReadBuffer {
					bf = append(bf, make([]byte, DefaultReadBuffer)...)
				} else if !flag {
					break cycle
				}
			}
		}
		if err != nil {
			switch {
			case errors.Is(err, io.EOF):
				break cycle
			case flag && isTimeout(err):
				if err = c.SetReadDeadline(time.Now().Add(c.recvBufferInterval)); err != nil {
					return nil, err
				}
				break cycle
			case retry.Count > 0:
				// 触发重试
				retry.Count--
				if retry.Interval == 0 {
					retry.Interval = DefaultRetryInterval
				}
				time.Sleep(retry.Interval)
				goto cycle
			default:
				return nil, err
			}
		}
		if length == 0 {
			break cycle
		}
	}
	return bf[:index], nil
}

// RecvLine 读取一行 '\n'
func (c *Conn) RecvLine(retry *Retry) ([]byte, error) {
	var (
		// err
		err error

		data []byte

		index int

		bf = (*buffer.GetBytes(1024))[:0]
	)
	for {
		data, err = c.Recv(1, retry)
		if err != nil || data[0] == '\n' {
			break
		}
		index++
		bf = append(bf, data...)
	}
	return bf[:index], err
}

// RecvWithTimeout 读取已经超时的链接
func (c *Conn) RecvWithTimeout(length int, timeout time.Duration, retry *Retry) ([]byte, error) {
	if err := c.SetRecvDeadline(time.Now().Add(timeout)); err != nil {
		return nil, err
	}
	defer c.SetRecvDeadline(time.Time{})
	return c.Recv(length, retry)
}

// SendWithTimeout 写入数据给已经超时的链接
func (c *Conn) SendWithTimeout(data []byte, timeout time.Duration, retry *Retry) error {
	if err := c.SetSendDeadline(time.Now().Add(timeout)); err != nil {
		return err
	}
	defer c.SetSendDeadline(time.Time{})
	return c.Send(data, retry)
}

// SendRecv 写入数据并读取返回
func (c *Conn) SendRecv(data []byte, length int, retry *Retry) ([]byte, error) {
	if err := c.Send(data, retry); err != nil {
		return nil, err
	}
	return c.Recv(length, retry)
}

// SendRecvWithTimeout 将数据写入并读出已经超时的链接
func (c *Conn) SendRecvWithTimeout(data []byte, timeout time.Duration, length int, retry *Retry) ([]byte, error) {
	if err := c.Send(data, retry); err != nil {
		return nil, err
	}
	return c.RecvWithTimeout(length, timeout, retry)
}

func (c *Conn) SetDeadline(t time.Time) error {
	err := c.Conn.SetDeadline(t)
	if err == nil {
		c.recvTimeout = t
		c.sendTimeout = t
	}
	return err
}

func (c *Conn) SetRecvDeadline(t time.Time) error {
	err := c.SetReadDeadline(t)
	if err == nil {
		c.sendTimeout = t
	}
	return err
}

func (c *Conn) SetSendDeadline(t time.Time) error {
	err := c.SetWriteDeadline(t)
	if err == nil {
		c.sendTimeout = t
	}
	return err
}

// isTimeout: 判断是否是超时的error错误
func isTimeout(err error) bool {
	if err == nil {
		return false
	}
	if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
		return true
	}
	return false
}

// RecoveryBuffer 用于回收已经不使用的 *[]byte
// 如果使用已经回收的资源,可能会造成panic,请注意
func RecoveryBuffer(data *[]byte) {
	buffer.PutBytes(data)
}

// SetRecvBufferInterval 读取缓存间隔时间
func (c *Conn) SetRecvBufferInterval(t time.Duration) {
	c.recvBufferInterval = t
}

// NewConnByNetConn 通过原始的 net.Conn 链接建立 Conn 封装对象
func NewConnByNetConn(conn net.Conn) *Conn {
	return &Conn{
		Conn:               conn,
		reader:             bufio.NewReader(conn),
		sendTimeout:        time.Time{},
		recvTimeout:        time.Time{},
		recvBufferInterval: DefaultWaitTimeout,
	}
}

// newNetConn 新建conn
func newNetConn(addr string, timeout *time.Duration) (net.Conn, error) {
	if timeout == nil {
		timeout = &DefaultConnTimeout
	}
	return net.DialTimeout("tcp", addr, *timeout)
}

// newNetConnTLS
func newNetConnTLS(addr string, tlsConfig *tls.Config, timeout *time.Duration) (net.Conn, error) {
	if timeout == nil {
		timeout = &DefaultConnTimeout
	}
	dialer := &net.Dialer{Timeout: *timeout}
	return tls.DialWithDialer(dialer, "tcp", addr, tlsConfig)
}

// NewConn 通过原始拨号建立
func NewConn(addr string, timeout *time.Duration) (*Conn, error) {
	if conn, err := newNetConn(addr, timeout); err != nil {
		return nil, err
	} else {
		return NewConnByNetConn(conn), nil
	}
}

// NewConnTLS 通过tls建立
func NewConnTLS(addr string, tlsConfig *tls.Config, timeout *time.Duration) (*Conn, error) {
	if conn, err := newNetConnTLS(addr, tlsConfig, timeout); err != nil {
		return nil, err
	} else {
		return NewConnByNetConn(conn), nil
	}
}
