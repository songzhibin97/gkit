package pool

import (
	"context"
	"time"
)

var pool Pool

type mock map[string]string

func (m *mock) Shutdown() error {
	return nil
}

// getResources: 获取资源,返回的资源对象需要实现 IShutdown 接口,用于资源回收
func getResources(c context.Context) (IShutdown, error) {
	return &mock{}, nil
}

func ExampleNewList() {
	// NewList(options ...)
	// 默认配置
	// pool = NewList()

	// 可供选择配置选项

	// 设置 Pool 连接数, 如果 == 0 则无限制
	// SetActive(100)

	// 设置最大空闲连接数
	// SetIdle(20)

	// 设置空闲等待时间
	// SetIdleTimeout(time.Second)

	// 设置期望等待
	// SetWait(false,time.Second)

	// 自定义配置
	pool = NewList(
		SetActive(100),
		SetIdle(20),
		SetIdleTimeout(time.Second),
		SetWait(false, time.Second))

	// New需要实例化,否则在 pool.Get() 会无法获取到资源
	pool.New(getResources)
}

func ExampleList_Get() {
	v, err := pool.Get(context.TODO())
	if err != nil {
		// 处理错误
	}
	// v 获取到的资源
	_ = v
}

func ExampleList_Put() {
	v, err := pool.Get(context.TODO())
	if err != nil {
		// 处理错误
	}

	// Put: 资源回收
	// forceClose: true 内部帮你调用 Shutdown回收, 否则判断是否是可回收,挂载到list上
	err = pool.Put(context.TODO(), v, false)
	if err != nil {
		// 处理错误
	}
}

func ExampleList_Shutdown() {
	// Shutdown 回收资源,关闭所有资源
	_ = pool.Shutdown()
}
