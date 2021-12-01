package concurrent

import "sync"

type WaitGroup interface {
	Add(int)
	Wait()
	Done()
	Do()
}

type OrderlyTask struct {
	sync.WaitGroup
	fn func()
}

// Do 执行任务
func (o *OrderlyTask) Do() {
	o.Add(1)
	go func() {
		defer o.Done()
		o.fn()
	}()
}

// NewOrderTask 初始化任务
func NewOrderTask(fn func()) *OrderlyTask {
	return &OrderlyTask{
		fn: fn,
	}
}

// Orderly 顺序执行
func Orderly(tasks []*OrderlyTask) {
	for _, task := range tasks {
		task.Do()
		task.Wait()
	}
}
