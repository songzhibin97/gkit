package pool

import (
	"container/list"
	"context"
	"io"
	"sync"
)

//var _ Pool = &List{}

// List:
type List struct {
	// f: item
	f func(ctx context.Context) (io.Closer, error)

	// mu: 互斥锁, 保护以下字段
	mu sync.Mutex

	// cond:
	cond chan struct{}

	// cleanerCh: 清空 ch
	cleanerCh chan struct{}

	// active: 最大连接数
	active uint64

	// conf: 配置信息
	conf *Config

	// closed:
	closed uint32

	// idles:
	idles list.List
}
