package tcp

import "time"

// Send 拨号并发送消息,关闭链接
func Send(addr string, data []byte, retry *Retry) error {
	c, err := NewConn(addr, nil)
	if err != nil {
		return err
	}
	defer c.Close()
	return c.Send(data, retry)
}

// SendRecv 拨号发送并读取响应
func SendRecv(addr string, data []byte, length int, retry *Retry) ([]byte, error) {
	conn, err := NewConn(addr, nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return conn.SendRecv(data, length, retry)
}

// SendWithTimeout 创建并发送具有写入超时限制的连接
func SendWithTimeout(addr string, data []byte, timeout time.Duration, retry *Retry) error {
	conn, err := NewConn(addr, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	return conn.SendWithTimeout(data, timeout, retry)
}

// SendRecvWithTimeout 创建链接发送读取有超时限制的连接
func SendRecvWithTimeout(addr string, data []byte, receive int, timeout time.Duration, retry *Retry) ([]byte, error) {
	conn, err := NewConn(addr, nil)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	return conn.SendRecvWithTimeout(data, timeout, receive, retry)
}
