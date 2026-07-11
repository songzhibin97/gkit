package tcp

import (
	"bufio"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"time"

	"github.com/songzhibin97/gkit/cache/buffer"
)

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

type retryWait func(time.Duration) error

func sleepForRetry(interval time.Duration) error {
	time.Sleep(interval)
	return nil
}

func sleepForRetryUntil(deadline time.Time) retryWait {
	return func(interval time.Duration) error {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return os.ErrDeadlineExceeded
		}
		if interval > remaining {
			interval = remaining
		}
		time.Sleep(interval)
		if !time.Now().Before(deadline) {
			return os.ErrDeadlineExceeded
		}
		return nil
	}
}

// Send 发送数据至对端,有重试机制
func (c *Conn) Send(data []byte, retry *Retry) error {
	return c.send(data, retry, sleepForRetry)
}

func (c *Conn) send(data []byte, retry *Retry, wait retryWait) error {
	if retry == nil {
		// Take a per-call zero-value Retry rather than mutating a package
		// global. The previous code stored `&defaultRetry` and then wrote
		// `retry.Count--` through it, corrupting state for every concurrent
		// caller that also passed nil.
		var local Retry
		retry = &local
	}
	for offset := 0; offset < len(data); {
		remaining := data[offset:]
		n, err := c.Write(remaining)
		if n < 0 || n > len(remaining) {
			return fmt.Errorf("tcp send: invalid write count %d for %d remaining bytes", n, len(remaining))
		}
		offset += n
		if err != nil {
			if retry.Count > 0 && offset < len(data) {
				retry.Count--
				if retry.Interval == 0 {
					retry.Interval = DefaultRetryInterval
				}
				if waitErr := wait(retry.Interval); waitErr != nil {
					return fmt.Errorf("tcp send retry wait after %d of %d bytes: %w", offset, len(data), waitErr)
				}
				continue
			}
			return fmt.Errorf("tcp send after %d of %d bytes: %w", offset, len(data), err)
		}
		if n == 0 {
			return fmt.Errorf("tcp send after %d of %d bytes: %w", offset, len(data), io.ErrNoProgress)
		}
	}
	return nil
}

// Recv 接受数据
// length == 0 从 Conn一次读取立即返回
// length < 0 从 Conn 接收所有数据，并将其返回，直到没有数据
// length > 0 从 Conn 接收到对应的数据返回
func (c *Conn) Recv(length int, retry *Retry) (result []byte, retErr error) {
	return c.recv(length, retry, sleepForRetry)
}

func (c *Conn) recv(length int, retry *Retry, wait retryWait) (result []byte, retErr error) {
	if retry == nil {
		var local Retry
		retry = &local
	}
	readWithRetry := func(dst []byte) (int, error) {
		for {
			n, err := c.reader.Read(dst)
			if err == nil || errors.Is(err, io.EOF) || retry.Count == 0 {
				return n, err
			}
			retry.Count--
			if retry.Interval == 0 {
				retry.Interval = DefaultRetryInterval
			}
			if waitErr := wait(retry.Interval); waitErr != nil {
				return n, waitErr
			}
			if n > 0 {
				// Preserve bytes returned with a retryable error. The caller
				// advances its offset and the next read retries the remainder.
				return n, nil
			}
		}
	}

	if length > 0 {
		bf := *buffer.GetBytes(length)
		index := 0
		for index < length {
			n, err := readWithRetry(bf[index:])
			index += n
			if index == length {
				return bf[:index], nil
			}
			if errors.Is(err, io.EOF) {
				if index == 0 {
					return bf[:0], io.EOF
				}
				return bf[:index], io.ErrUnexpectedEOF
			}
			if err != nil {
				return bf[:index], fmt.Errorf("tcp receive %d of %d bytes: %w", index, length, err)
			}
			if n == 0 {
				return bf[:index], fmt.Errorf("tcp receive %d of %d bytes: %w", index, length, io.ErrNoProgress)
			}
		}
		return bf[:index], nil
	}

	bufferSize := DefaultReadBuffer
	if bufferSize <= 0 {
		bufferSize = 1
	}
	bf := *buffer.GetBytes(bufferSize)
	if length == 0 {
		n, err := readWithRetry(bf)
		if errors.Is(err, io.EOF) {
			return bf[:n], nil
		}
		return bf[:n], err
	}

	previousDeadline := c.recvTimeout
	deadlineChanged := false
	defer func() {
		if !deadlineChanged {
			return
		}
		if err := c.Conn.SetReadDeadline(previousDeadline); err != nil {
			restoreErr := fmt.Errorf("tcp receive: restore read deadline: %w", err)
			if retErr == nil {
				retErr = restoreErr
			} else {
				retErr = errors.Join(retErr, restoreErr)
			}
		}
	}()

	index := 0
	for {
		if index == len(bf) {
			bf = append(bf, make([]byte, bufferSize)...)
		}
		n, err := readWithRetry(bf[index:])
		index += n
		if errors.Is(err, io.EOF) {
			return bf[:index], nil
		}
		if err != nil {
			if deadlineChanged && isTimeout(err) {
				return bf[:index], nil
			}
			return bf[:index], fmt.Errorf("tcp receive stream after %d bytes: %w", index, err)
		}
		if n == 0 {
			return bf[:index], fmt.Errorf("tcp receive stream after %d bytes: %w", index, io.ErrNoProgress)
		}
		idleDeadline := time.Now().Add(c.recvBufferInterval)
		if !previousDeadline.IsZero() && previousDeadline.Before(idleDeadline) {
			idleDeadline = previousDeadline
		}
		if err := c.Conn.SetReadDeadline(idleDeadline); err != nil {
			return bf[:index], fmt.Errorf("tcp receive stream: set idle deadline: %w", err)
		}
		deadlineChanged = true
	}
}

// RecvLine 读取一行 '\n'
func (c *Conn) RecvLine(retry *Retry) ([]byte, error) {
	bf := (*buffer.GetBytes(1024))[:0]
	for {
		data, err := c.Recv(1, retry)
		if len(data) > 0 {
			if data[0] == '\n' {
				return bf, nil
			}
			bf = append(bf, data...)
		}
		if err != nil {
			return bf, err
		}
		if len(data) == 0 {
			return bf, io.ErrNoProgress
		}
	}
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
	return c.sendRecv(data, length, retry, sleepForRetry)
}

func (c *Conn) sendRecv(data []byte, length int, retry *Retry, wait retryWait) ([]byte, error) {
	if err := c.send(data, retry, wait); err != nil {
		return nil, err
	}
	return c.recv(length, retry, wait)
}

// SendRecvWithTimeout 将数据写入并读出已经超时的链接
func (c *Conn) SendRecvWithTimeout(data []byte, timeout time.Duration, length int, retry *Retry) (result []byte, retErr error) {
	deadline := time.Now().Add(timeout)
	if err := c.SetDeadline(deadline); err != nil {
		return nil, fmt.Errorf("tcp send receive: set deadline: %w", err)
	}
	defer func() {
		if err := c.SetDeadline(time.Time{}); err != nil {
			clearErr := fmt.Errorf("tcp send receive: clear deadline: %w", err)
			if retErr == nil {
				retErr = clearErr
				return
			}
			retErr = errors.Join(retErr, clearErr)
		}
	}()
	return c.sendRecv(data, length, retry, sleepForRetryUntil(deadline))
}

func (c *Conn) SetDeadline(t time.Time) error {
	err := c.Conn.SetDeadline(t)
	if err == nil {
		c.recvTimeout = t
		c.sendTimeout = t
	}
	return err
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	err := c.Conn.SetReadDeadline(t)
	if err == nil {
		c.recvTimeout = t
	}
	return err
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	err := c.Conn.SetWriteDeadline(t)
	if err == nil {
		c.sendTimeout = t
	}
	return err
}

func (c *Conn) SetRecvDeadline(t time.Time) error {
	err := c.SetReadDeadline(t)
	return err
}

func (c *Conn) SetSendDeadline(t time.Time) error {
	err := c.SetWriteDeadline(t)
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
