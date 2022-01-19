package codel

import (
	"context"

	"github.com/songzhibin97/gkit/overload/bbr"
)

var queue *Queue

func ExampleNew() {
	// 默认配置
	// queue = NewQueue()

	// 可供选择配置选项

	// 设置对列延时
	// SetTarget(40)

	// 设置滑动窗口最小时间宽度
	// SetInternal(1000)

	queue = NewQueue(SetTarget(40), SetInternal(1000))
}

func ExampleQueue_Stat() {
	// start 体现 CoDel 状态信息
	start := queue.Stat()

	_ = start
}

func ExampleQueue_Push() {
	// 入队
	if err := queue.Push(context.TODO()); err != nil {
		if err == bbr.LimitExceed {
			// todo 处理过载保护错误
		} else {
			// todo 处理其他错误
		}
	}
}

func ExampleQueue_Pop() {
	// 出队,没有请求则会阻塞
	queue.Pop()
}
