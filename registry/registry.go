package registry

import "context"

// package registry: 服务注册发现

// Registrar 注册抽象
type Registrar interface {
	// Register 注册
	Register(ctx context.Context, service *ServiceInstance) error
	// Deregister 注销
	Deregister(ctx context.Context, service *ServiceInstance) error
}

// Discovery 服务发现抽象
type Discovery interface {
	// GetService 返回服务名相关的服务实例
	GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	// Watch 根据服务名创建监控
	Watch(ctx context.Context, serviceName string) (Watcher, error)
}

// Watcher 服务监控
// Watch需要满足以下条件
// 1. 第一次 GetService 的列表不为空
// 2. 发现任何服务实例更改
// 不满足以上两种条件,Next则会无限等待直到上下文截止
type Watcher interface {
	Next() ([]*ServiceInstance, error)
	// Stop 停止监控行为
	Stop() error
}
