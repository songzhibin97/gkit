package pool

import (
	"Songzhibin/GKit/timeout"
	"container/list"
	"context"
	"sync"
	"sync/atomic"
	"time"
)

//var _ Pool = &List{}

// List:
type List struct {
	// f: item
	f func(ctx context.Context) (Shutdown, error)

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

// NewList: 实例化
func NewList(c *Config) *List {
	if c == nil || c.Active < c.Idle {
		panic("config nil或Idle必须<=有效")
	}
	p := &List{conf: c}
	p.cond = make(chan struct{})

	return p
}

// Reload: 重新加载
func (l *List) Reload(c *Config) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.conf = c
}

// startCleanerLocked
func (l *List) startCleanerLocked(d time.Duration) {
	// 如果 <= 0 放弃设置
	if d <= 0 {
		return
	}
	// 如果时间间隔d小于等待超时,并且 cleanerCh 不为nil 监听信号
	if d < l.conf.IdleTimeout && l.cleanerCh != nil {
		select {
		case l.cleanerCh <- struct{}{}:
		default:
		}
	}
	// 懒加载
	if l.cleanerCh == nil {
		l.cleanerCh = make(chan struct{}, 1)
		go l.staleCleaner()
	}

}

// staleCleaner:
func (l *List) staleCleaner() {
	ticker := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-ticker.C:
		case <-l.cleanerCh:
			// 重新加载配置
		}
		l.mu.Lock()
		// 是否关闭 或者 没有设置超时时间
		if atomic.LoadUint32(&l.closed) == 1 || l.conf.IdleTimeout <= 0 {
			l.mu.Unlock()
			return
		}
		for i, n := 0, l.idles.Len(); i < n; i++ {
			e := l.idles.Back()
			if e == nil {
				break
			}
			ic := e.Value.(item)
			if !ic.expire(l.conf.IdleTimeout) {
				break
			}
			l.idles.Remove(e)
			l.release()
			l.mu.Unlock()
			_ = ic.c.Shutdown()
			l.mu.Lock()
		}
		l.mu.Unlock()
	}
}

// release decrements the active count and signals waiters. The caller must
// hold p.mu during the call.
func (l *List) release() {
	l.active--
	l.signal()
}

// signal:
func (l *List) signal() {
	select {
	default:
	case l.cond <- struct{}{}:
	}
}

// Get:
func (l *List) Get(ctx context.Context) (Shutdown, error) {
	l.mu.Lock()
	// 判断是否关闭
	if atomic.LoadUint32(&l.closed) == 1 {
		l.mu.Unlock()
		return nil, ErrPoolClosed
	}
	for {
		for i, n := 0, l.idles.Len(); i < n; i++ {

			e := l.idles.Back()
			if e == nil {
				break
			}
			ic := e.Value.(item)
			l.idles.Remove(e)
			l.mu.Unlock()
			if !ic.expire(l.conf.IdleTimeout) {
				return ic.c, nil
			}
			ic.c.Shutdown()
			l.mu.Lock()
			l.release()
		}

		// 检查是否关闭
		if atomic.LoadUint32(&l.closed) == 1 {
			return nil, ErrPoolClosed
		}
		if l.conf.Active == 0 || l.active < l.conf.Active {
			newItem := l.f
			l.active++
			l.mu.Unlock()
			c, err := newItem(ctx)
			if err != nil {
				l.mu.Lock()
				l.release()
				l.mu.Unlock()
				c = nil
			}
			return c, err
		}
		if l.conf.WaitTimeout == 0 || !l.conf.Wait {
			l.mu.Unlock()
			return nil, ErrPoolExhausted
		}
		wt := l.conf.WaitTimeout
		l.mu.Unlock()

		//
		nctx := ctx
		cancel := func() {}
		if wt > 0 {
			_, nctx, cancel = timeout.Shrink(ctx, wt)
		}
		select {
		case <-nctx.Done():
			cancel()
			return nil, nctx.Err()
		case <-l.cond:
		}
		cancel()
		l.mu.Lock()
	}
}

// Put:
func (l *List) Put(ctx context.Context, c Shutdown, forceClose bool) error {
	l.mu.Lock()
	if atomic.LoadUint32(&l.closed) == 1 && !forceClose {
		l.idles.PushFront(item{createdAt: nowFunc(), c: c})
		if uint64(l.idles.Len()) > l.conf.Idle {
			c = l.idles.Remove(l.idles.Back()).(item).c
		} else {
			c = nil
		}
	}
	if c == nil {
		l.signal()
		l.mu.Unlock()
		return nil
	}
	l.release()
	l.mu.Unlock()
	return c.Shutdown()
}

// Shutdown:
func (l *List) Shutdown() error {
	l.mu.Lock()
	idles := l.idles
	l.idles.Init()
	atomic.StoreUint32(&l.closed, 1)
	l.active -= uint64(idles.Len())
	l.mu.Unlock()
	for e := idles.Front(); e != nil; e = e.Next() {
		_ = e.Value.(item).c.Shutdown()
	}
	return nil
}
