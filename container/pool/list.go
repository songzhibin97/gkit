package pool

import (
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/gkit/options"
	"github.com/songzhibin97/gkit/timeout"
)

var _ Pool = &List{}

// List 双向链表
type List struct {
	// f: item
	f func(ctx context.Context) (IShutdown, error)

	// mu: 互斥锁, 保护以下字段
	mu sync.Mutex

	// generation is closed and replaced under mu whenever pool state changes
	// may allow a waiter to make progress.
	generation chan struct{}

	// cleanerWake resets the cleaner after an idle timeout reload. It remains
	// open for the lifetime of List so Reload cannot send to a closed channel.
	cleanerWake chan struct{}

	// cleanerStop is closed exactly once by the first successful Shutdown.
	cleanerStop chan struct{}

	// cleanerDone is closed after the single cleaner goroutine has stopped.
	cleanerDone chan struct{}

	// cleanerOnce prevents duplicate cleaner goroutines if Timer is called by
	// code outside NewList.
	cleanerOnce sync.Once

	// active: 最大连接数
	active uint64

	// conf: 配置信息
	conf *config

	// closed:
	closed uint32

	// idles: 链表
	idles list.List
}

// Reload 重新设置配置文件
func (l *List) Reload(options ...options.Option) {
	l.mu.Lock()
	defer l.mu.Unlock()
	previousIdleTimeout := l.conf.idleTimeout
	for _, option := range options {
		option(l.conf)
	}
	l.notifyLocked()
	if l.conf.idleTimeout != previousIdleTimeout {
		l.wakeCleaner()
	}
}

// Init 初始化
func (l *List) Init(d time.Duration) {
	if d <= 0 {
		return
	}
	l.wakeCleaner()
}

// wakeCleaner requests an immediate cleaner pass and timer reset. The channel
// is buffered so Reload never waits for the cleaner, and it is intentionally
// never closed so calls racing with or following Shutdown remain safe.
func (l *List) wakeCleaner() {
	select {
	case l.cleanerWake <- struct{}{}:
	default:
	}
}

// Timer 定时任务
func (l *List) Timer(_ time.Duration) {
	l.cleanerOnce.Do(l.runCleaner)
}

func (l *List) runCleaner() {
	defer close(l.cleanerDone)

	timer := time.NewTimer(time.Hour)
	stopAndDrainTimer(timer)
	defer stopAndDrainTimer(timer)

	for {
		l.mu.Lock()
		idleTimeout := l.conf.idleTimeout
		l.mu.Unlock()

		var timerC <-chan time.Time
		if idleTimeout > 0 {
			if idleTimeout < minDuration {
				idleTimeout = minDuration
			}
			stopAndDrainTimer(timer)
			timer.Reset(idleTimeout)
			timerC = timer.C
		} else {
			stopAndDrainTimer(timer)
		}

		select {
		case <-timerC:
		case <-l.cleanerWake:
		case <-l.cleanerStop:
			return
		}

		l.cleanExpired()
	}
}

func stopAndDrainTimer(timer *time.Timer) {
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

// cleanExpired removes every expired idle from the live list while holding mu,
// then shuts the detached resources down off-lock. Shutdown can therefore
// snapshot only the remaining idles and each resource is closed exactly once.
func (l *List) cleanExpired() {
	l.mu.Lock()
	if atomic.LoadUint32(&l.closed) == 1 || l.conf.idleTimeout <= 0 {
		l.mu.Unlock()
		return
	}

	idleTimeout := l.conf.idleTimeout
	shutdowns := make([]IShutdown, 0)
	for {
		e := l.idles.Back()
		if e == nil {
			break
		}
		idle := e.Value.(item)
		if !idle.expire(idleTimeout) {
			break
		}
		l.idles.Remove(e)
		shutdowns = append(shutdowns, idle.s)
	}
	if len(shutdowns) > 0 {
		atomic.AddUint64(&l.active, ^uint64(len(shutdowns)-1))
		l.notifyLocked()
	}
	l.mu.Unlock()

	for _, shutdown := range shutdowns {
		_ = shutdown.Shutdown()
	}
}

// releaseLocked atomically decrements the active count and broadcasts a state
// change. The caller must hold l.mu.
func (l *List) releaseLocked() {
	// l.active -= 1
	atomic.AddUint64(&l.active, ^uint64(0))
	l.notifyLocked()
}

// releaseUnlocked is the release path for callers that do not hold l.mu.
func (l *List) releaseUnlocked() {
	l.mu.Lock()
	l.releaseLocked()
	l.mu.Unlock()
}

// notifyLocked broadcasts a state change without losing notifications that
// happen before a waiter starts selecting. The caller must hold l.mu.
func (l *List) notifyLocked() {
	close(l.generation)
	l.generation = make(chan struct{})
}

// Get 获取
func (l *List) Get(ctx context.Context) (IShutdown, error) {
	l.mu.Lock()
	// 判断是否关闭
	if atomic.LoadUint32(&l.closed) == 1 {
		l.mu.Unlock()
		return nil, ErrPoolClosed
	}
	for {
		for i, n := 0, l.idles.Len(); i < n; i++ {
			e := l.idles.Front()
			if e == nil {
				break
			}
			ic := e.Value.(item)
			l.idles.Remove(e)
			idleTimeout := l.conf.idleTimeout
			l.mu.Unlock()
			// 没有过期的可以直接返回了
			if !ic.expire(idleTimeout) {
				return ic.s, nil
			}
			// 清理 重新获取锁
			_ = ic.s.Shutdown()
			l.mu.Lock()
			l.releaseLocked()
		}

		// 检查是否关闭
		if atomic.LoadUint32(&l.closed) == 1 {
			l.mu.Unlock()
			return nil, ErrPoolClosed
		}
		// 判断是否需要新增
		if l.conf.active == 0 || atomic.LoadUint64(&l.active) < l.conf.active {
			if l.f == nil {
				l.mu.Unlock()
				return nil, ErrPoolNewFuncIsNull
			}
			newItem := l.f
			// Bump the active counter under the lock so two concurrent
			// Gets that both pass the capacity check cannot also both
			// increment past `conf.active`. The previous code released
			// the lock first, then `atomic.AddUint64`, allowing N goroutines
			// to all observe pre-increment values.
			atomic.AddUint64(&l.active, 1)
			l.mu.Unlock()
			c, err := newItem(ctx)
			if err != nil {
				l.releaseUnlocked()
				c = nil
			}
			return c, err
		}
		// 如果满了判断是否需要等待
		if l.conf.waitTimeout == 0 && !l.conf.wait {
			l.mu.Unlock()
			return nil, ErrPoolExhausted
		}
		// Capture the current generation while holding mu. A state change that
		// happens after unlock closes this channel, even if select has not begun.
		generation := l.generation
		wait := l.conf.wait
		wt := l.conf.waitTimeout
		l.mu.Unlock()

		if wait && wt == 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-generation:
			}
		} else {
			// 控制链路超时时间
			_, nCtx, cancel := timeout.Shrink(ctx, wt)
			select {
			case <-nCtx.Done():
				err := nCtx.Err()
				cancel()
				return nil, err
			case <-generation:
				cancel()
			}
		}
		// 自旋,再次尝试获得句柄
		l.mu.Lock()
	}
}

// Put 回收
func (l *List) Put(ctx context.Context, s IShutdown, forceClose bool) error {
	l.mu.Lock()
	if atomic.LoadUint32(&l.closed) == 0 && !forceClose {
		// 插入到链表头
		l.idles.PushFront(item{createdAt: nowFunc(), s: s})
		// 判断闲置数量是否达到阈值
		if uint64(l.idles.Len()) > l.conf.idle {
			// 拿到尾部淘汰的 shutdown
			s = l.idles.Remove(l.idles.Back()).(item).s
		} else {
			s = nil
		}
	}
	// 如果 s == nil 进入回收
	if s == nil {
		l.notifyLocked()
		l.mu.Unlock()
		return nil
	}
	l.releaseLocked()
	l.mu.Unlock()
	return s.Shutdown()
}

// Shutdown 关闭
func (l *List) Shutdown() error {
	l.mu.Lock()
	if atomic.SwapUint32(&l.closed, 1) == 1 {
		l.mu.Unlock()
		return ErrPoolClosed
	}
	close(l.cleanerStop)
	// container/list.List does not support copy-by-value: a copy of the
	// sentinel root carries pointers back to elements whose `list` field
	// still references the original. Snapshot the IShutdowns into a slice
	// while we hold the lock, then Init the live list. Iterating the
	// slice afterwards is safe without locks.
	shutdowns := make([]IShutdown, 0, l.idles.Len())
	for e := l.idles.Front(); e != nil; e = e.Next() {
		shutdowns = append(shutdowns, e.Value.(item).s)
	}
	if len(shutdowns) > 0 {
		atomic.AddUint64(&l.active, ^uint64(len(shutdowns)-1))
	}
	l.idles.Init()
	l.notifyLocked()
	l.mu.Unlock()
	for _, s := range shutdowns {
		_ = s.Shutdown()
	}
	<-l.cleanerDone
	return nil
}

// New 设置创建资源函数
func (l *List) New(f func(ctx context.Context) (IShutdown, error)) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.f = f
}

// NewList 实例化
func NewList(options ...options.Option) Pool {
	l := &List{
		conf:        defaultConfig(),
		generation:  make(chan struct{}),
		cleanerWake: make(chan struct{}, 1),
		cleanerStop: make(chan struct{}),
		cleanerDone: make(chan struct{}),
	}
	for _, option := range options {
		option(l.conf)
	}
	// The cleaner exists even when cleanup starts disabled so a later Reload can
	// enable it without spawning a second goroutine.
	go l.Timer(l.conf.idleTimeout)
	return l
}
