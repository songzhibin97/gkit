package tcp

import "time"

var (
	// DefaultWaitTimeout 默认等待超时时间
	DefaultWaitTimeout = time.Millisecond

	// DefaultConnTimeout 默认连接超时时间
	DefaultConnTimeout = 30 * time.Second

	// DefaultRetryInterval 默认重试间隔
	DefaultRetryInterval = 100 * time.Millisecond

	// DefaultReadBuffer 默认读取buffer大小
	DefaultReadBuffer = 1 << 12

	// DefaultServer 默认的服务名称
	DefaultServer = "Default"
)

const (
	// Status

	UNKNOWN = iota
	ACTIVE
	ERROR
)
