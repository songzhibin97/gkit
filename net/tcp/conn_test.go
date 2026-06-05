package tcp

import (
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConn_Send(t *testing.T) {
	connCh := make(chan net.Conn, 1)
	ok := make(chan struct{}, 1)
	// run server
	go func() {
		l, err := net.Listen("tcp", "127.0.0.1:8999")
		assert.NoError(t, err)
		defer l.Close()
		ok <- struct{}{}
		c, err := l.Accept()
		assert.NoError(t, err)
		connCh <- c
	}()
	<-ok
	// client
	mockConn, err := NewConn("127.0.0.1:8999", nil)
	assert.NoError(t, err)
	defer mockConn.Close()
	body := []byte("hello world")
	err = mockConn.Send(body, nil)
	assert.NoError(t, err)

	readBody := make([]byte, len(body))
	conn := <-connCh
	n, err := conn.Read(readBody)
	assert.NoError(t, err)
	assert.Equal(t, n, len(body))
	assert.Equal(t, body, readBody)

	t.Log("send:", string(body), "->", "read:", string(readBody))
}

func TestConn_Recv(t *testing.T) {
	connCh := make(chan net.Conn, 1)
	ok := make(chan struct{}, 1)
	DefaultReadBuffer = 16
	// run server
	go func() {
		l, err := net.Listen("tcp", "127.0.0.1:8999")
		assert.NoError(t, err)
		defer l.Close()
		ok <- struct{}{}
		c, err := l.Accept()
		assert.NoError(t, err)
		connCh <- c
	}()
	<-ok
	// client
	mockConn, err := NewConn("127.0.0.1:8999", nil)
	assert.NoError(t, err)
	defer mockConn.Close()

	body := []byte("hello world")
	conn := <-connCh
	{
		// 正常一次收发
		n, err := conn.Write(body)
		assert.NoError(t, err)
		assert.Equal(t, n, len(body))

		readBody, err := mockConn.Recv(0, nil)
		assert.NoError(t, err)
		assert.Equal(t, body, readBody)

		t.Log("send:", string(body), "->", "read:", string(readBody))
	}

	{
		// 指定长度
		n, err := conn.Write(body)
		assert.NoError(t, err)
		assert.Equal(t, n, len(body))

		readBody, err := mockConn.Recv(8, nil)
		assert.NoError(t, err)
		assert.Equal(t, body[:8], readBody)

		t.Log("send:", string(body[:8]), "->", "read:", string(readBody))
		_, _ = mockConn.Recv(0, nil)
	}

	{
		body2 := []byte("012345678910")
		// 指定长度

		n, err := conn.Write(body)
		assert.NoError(t, err)
		assert.Equal(t, n, len(body))

		n, err = conn.Write(body2)
		assert.NoError(t, err)
		assert.Equal(t, n, len(body2))

		readBody, err := mockConn.Recv(20, nil)
		assert.NoError(t, err)

		assert.Equal(t, []byte("hello world012345678"), readBody)

		t.Log("send:", "hello world012345678", "->", "read:", string(readBody))
	}

	{
		readBody, err := mockConn.Recv(-1, nil)
		assert.NoError(t, err)
		assert.Equal(t, []byte("910"), readBody)
		t.Log("send:", "910", "->", "read:", string(readBody))

	}
}

// TestConn_SetDeadlineFields 回归测试:确保 SetRecvDeadline 写入 recvTimeout 而非 sendTimeout
// (历史上曾因复制粘贴 bug 写错字段)
func TestConn_SetDeadlineFields(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	assert.NoError(t, err)
	defer l.Close()

	go func() {
		c, err := l.Accept()
		if err == nil {
			defer c.Close()
		}
	}()

	mockConn, err := NewConn(l.Addr().String(), nil)
	assert.NoError(t, err)
	defer mockConn.Close()

	zero := time.Time{}

	// SetRecvDeadline 只更新 recvTimeout
	recvDL := time.Now().Add(5 * time.Second)
	assert.NoError(t, mockConn.SetRecvDeadline(recvDL))
	assert.Equal(t, recvDL, mockConn.recvTimeout, "SetRecvDeadline 应更新 recvTimeout")
	assert.Equal(t, zero, mockConn.sendTimeout, "SetRecvDeadline 不应影响 sendTimeout")

	// SetSendDeadline 只更新 sendTimeout
	sendDL := time.Now().Add(7 * time.Second)
	assert.NoError(t, mockConn.SetSendDeadline(sendDL))
	assert.Equal(t, recvDL, mockConn.recvTimeout, "SetSendDeadline 不应影响 recvTimeout")
	assert.Equal(t, sendDL, mockConn.sendTimeout, "SetSendDeadline 应更新 sendTimeout")

	// SetDeadline 同时更新两者
	bothDL := time.Now().Add(9 * time.Second)
	assert.NoError(t, mockConn.SetDeadline(bothDL))
	assert.Equal(t, bothDL, mockConn.recvTimeout)
	assert.Equal(t, bothDL, mockConn.sendTimeout)
}

func TestConn_RecvLine(t *testing.T) {
	connCh := make(chan net.Conn, 1)
	ok := make(chan struct{}, 1)
	DefaultReadBuffer = 16
	// run server
	go func() {
		l, err := net.Listen("tcp", "127.0.0.1:8999")
		assert.NoError(t, err)
		defer l.Close()
		ok <- struct{}{}
		c, err := l.Accept()
		assert.NoError(t, err)
		connCh <- c
	}()
	<-ok

	// client
	mockConn, err := NewConn("127.0.0.1:8999", nil)
	assert.NoError(t, err)
	defer mockConn.Close()

	body := `测试readline
测试Gkit
测试Debug
`
	conn := <-connCh
	n, err := conn.Write([]byte(body))
	assert.NoError(t, err)
	assert.Equal(t, n, len(body))
	{
		readBody, err := mockConn.RecvLine(nil)
		assert.NoError(t, err)
		assert.Equal(t, []byte("测试readline"), readBody)
		t.Log("send:", "测试readline", "->", "read:", string(readBody))
	}
	{
		readBody, err := mockConn.RecvLine(nil)
		assert.NoError(t, err)
		assert.Equal(t, []byte("测试Gkit"), readBody)
		t.Log("send:", "测试Gkit", "->", "read:", string(readBody))
	}
	{
		readBody, err := mockConn.RecvLine(nil)
		assert.NoError(t, err)
		assert.Equal(t, []byte("测试Debug"), readBody)
		t.Log("send:", "测试Debug", "->", "read:", string(readBody))
	}
}
