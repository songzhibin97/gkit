package goroutine

import (
	"context"
	"time"

	"github.com/songzhibin97/gkit/log"
)

var gGroup GGroup

func mockFunc() func() {
	return func() {
	}
}

func ExampleNewGoroutine() {
	// 默认配置
	// gGroup = NewGoroutine(context.TODO())

	// 可供选择配置选项

	// 设置停止超时时间
	// SetStopTimeout(time.Second)

	// 设置日志对象
	// SetLogger(&testLogger{})

	// 设置pool最大容量
	// SetMax(100)

	gGroup = NewGoroutine(context.TODO(),
		SetStopTimeout(time.Second),
		SetLogger(log.DefaultLogger),
		SetMax(100),
	)
}

func ExampleGoroutine_AddTask() {
	if !gGroup.AddTask(mockFunc()) {
		// 添加任务失败
	}
}

func ExampleGoroutine_AddTaskN() {
	// 带有超时控制添加任务
	if !gGroup.AddTaskN(context.TODO(), mockFunc()) {
		// 添加任务失败
	}
}

func ExampleGoroutine_ChangeMax() {
	// 修改 pool最大容量
	gGroup.ChangeMax(1000)
}

func ExampleGoroutine_Shutdown() {
	// 回收资源
	_ = gGroup.Shutdown()
}
