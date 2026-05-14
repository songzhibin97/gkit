package broker

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/songzhibin97/gkit/distributed/retry"
	"github.com/songzhibin97/gkit/options"
)

// registeredTask 注册的任务
type registeredTask struct {
	sync.RWMutex
	item map[string]bool
}

// Register 注册
func (r *registeredTask) Register(taskName string) {
	r.Lock()
	defer r.Unlock()
	r.item[taskName] = true
}

// RegisterList 注册
func (r *registeredTask) RegisterList(taskNameList ...string) {
	r.Lock()
	defer r.Unlock()
	for _, taskName := range taskNameList {
		r.item[taskName] = true
	}
}

// Quit 注销
func (r *registeredTask) Quit(taskName string) {
	r.Lock()
	defer r.Unlock()
	delete(r.item, taskName)
}

// IsRegister 是否注册
func (r *registeredTask) IsRegister(taskName string) bool {
	r.RLock()
	defer r.RUnlock()
	return r.item[taskName]
}

// NewRegisteredTask 初始化任务注册器
func NewRegisteredTask() *registeredTask {
	return &registeredTask{
		item: make(map[string]bool),
	}
}

// Broker Broker
type Broker struct {
	// registeredTask 注册器
	*registeredTask
	// retry 是否重试 — atomic.Bool so StopConsuming's flip and GetRetry's
	// reads never race. Previously a plain bool produced a data race the
	// race detector flagged whenever a consumer was stopped while workers
	// still polled GetRetry.
	retry atomic.Bool
	// retryFn 重试函数
	retryFn     func(ctx context.Context)
	retryCtx    context.Context
	retryCancel context.CancelFunc

	stopCtx    context.Context
	stopCancel context.CancelFunc
}

// SetRetry is used by options.Option callbacks to seed the retry flag at
// construction. After NewBroker returns, the flag is only mutated by
// StopConsuming; both reads and writes go through atomic.Bool.
func (b *Broker) SetRetry(v bool) { b.retry.Store(v) }

// NewBroker 初始化 Broker
func NewBroker(r *registeredTask, ctx context.Context, options ...options.Option) *Broker {
	b := &Broker{
		registeredTask: r,
	}
	for _, option := range options {
		option(b)
	}
	if b.retry.Load() && b.retryFn == nil {
		b.retryFn = retry.Retry()
	}
	b.retryCtx, b.retryCancel = context.WithCancel(ctx)
	b.stopCtx, b.stopCancel = context.WithCancel(ctx)
	return b
}

func (b *Broker) GetRetry() bool {
	return b.retry.Load()
}

func (b *Broker) GetRetryFn() func(ctx context.Context) {
	return b.retryFn
}

func (b *Broker) GetRetryCtx() context.Context {
	return b.retryCtx
}

func (b *Broker) GetStopCtx() context.Context {
	return b.stopCtx
}

func (b *Broker) StopConsuming() {
	b.retry.Store(false)
	b.retryCancel()
	b.stopCancel()
}
