package pool

import (
	"context"
	"sync"
)

// Slice:
type Slice struct {
	// f: item
	f func(ctx context.Context) (IShutdown, error)

	// cancel: 关闭链路
	cancel context.CancelFunc

	// mu: 互斥锁, 保护以下字段
	mu sync.Mutex

	// itemRequests:
	itemRequests map[uint64]chan item

	// nextIndex: itemRequests 使用的下一个 key
	nextIndex uint64

	// active: 待处理的任务数
	active uint64

	// openerCh:
	openerCh  chan struct{}

	// cleanerCh: 清空 ch
	cleanerCh chan struct{}

	// conf: 配置信息
	conf *Config

	// closed:
	closed uint32

	// freeItems: 空闲的 items 对列
	freeItems []*item
}
