package window

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// package window: 提供环形窗口统计

// Index: 指标信息
type Index struct {
	Name  string
	Score uint
}

type Conf struct {
	// size: 窗口大小
	size uint

	// interval: 时间间隔
	interval time.Duration

	ctx context.Context
}

// Window: 窗口对象
type Window struct {
	// sync.Mutex: 互斥锁
	sync.Mutex

	// Conf: 配置信息
	Conf

	// index: 指针指向的位置
	index uint

	// cancel: 关闭函数
	cancel context.CancelFunc

	// close: 是否关闭
	close uint32


	// buffer: 根据窗口大小生成的buffer数组
	// map[string]uint
	buffer []atomic.Value

	// total: 所有指标
	total map[string]uint
}

// Sentinel: 初始化window对象后 后台开始滚动计数并同步更新到total
func (w *Window) Sentinel() {
	tick := time.Tick(w.interval)
	for {
		select {
		case _, ok := <-tick:
			if !ok {
				// 退出
				return
			}
			w.Lock()
			// 先收集上一次buffer的数据
			m := w.buffer[w.index].Load().(map[string]uint)
			for k, v := range m {
				w.total[k] += v
			}

			index := (w.index + 1) % w.size
			m = w.buffer[index].Load().(map[string]uint)
			// 清空原来的数据
			for k, v := range m {
				w.total[k] -= v
				if w.total[k] <= 0 {
					delete(w.total, k)
				}
			}
			w.buffer[index].Store(make(map[string]uint))
			// 最后在赋值
			w.index = index
			w.Unlock()
		case <-w.ctx.Done():
			return
		}
	}
}

// Shutdown: 关闭
func (w *Window) Shutdown() {
	if atomic.SwapUint32(&w.close, 1) == 1 {
		// 已经执行过close了
		return
	}
	w.cancel()
}

// AddIndex: 添加指标
func (w *Window) AddIndex(k string, v uint) {
	index := w.index
	m := w.buffer[index].Load().(map[string]uint)
	n := make(map[string]uint)
	for s, u := range m {
		n[s] = u
	}
	n[k] += v
	w.buffer[index].Store(n)
}

// Show: 展示total
func (w *Window) Show() []Index {
	res := make([]Index, 0, len(w.total))
	w.Lock()
	defer w.Unlock()
	for s, u := range w.total {
		res = append(res, Index{
			Name:  s,
			Score: u,
		})
	}
	return res
}
