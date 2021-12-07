package tcp

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConn_Send(t *testing.T) {
	var conn net.Conn
	ok := make(chan struct{}, 1)
	// run server
	go func() {
		l, err := net.Listen("tcp", "127.0.0.1:8999")
		assert.NoError(t, err)
		defer l.Close()
		ok <- struct{}{}
		conn, err = l.Accept()
		assert.NoError(t, err)
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
	for conn == nil {
	}
	n, err := conn.Read(readBody)
	assert.NoError(t, err)
	assert.Equal(t, n, len(body))
	assert.Equal(t, body, readBody)

	t.Log("send:", string(body), "->", "read:", string(readBody))
}

func TestConn_Recv(t *testing.T) {
	var conn net.Conn
	ok := make(chan struct{}, 1)
	DefaultReadBuffer = 16
	// run server
	go func() {
		l, err := net.Listen("tcp", "127.0.0.1:8999")
		assert.NoError(t, err)
		defer l.Close()
		ok <- struct{}{}
		conn, err = l.Accept()
		assert.NoError(t, err)
	}()
	<-ok
	// client
	mockConn, err := NewConn("127.0.0.1:8999", nil)
	assert.NoError(t, err)
	defer mockConn.Close()

	body := []byte("hello world")
	for conn == nil {
	}
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

func TestConn_RecvLine(t *testing.T) {
	var conn net.Conn
	ok := make(chan struct{}, 1)
	DefaultReadBuffer = 16
	// run server
	go func() {
		l, err := net.Listen("tcp", "127.0.0.1:8999")
		assert.NoError(t, err)
		defer l.Close()
		ok <- struct{}{}
		conn, err = l.Accept()
		assert.NoError(t, err)
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
	for conn == nil {
	}
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
