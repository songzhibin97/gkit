package middleware

import "context"

// package middleware: 封装中间件格式

// Endpoint
// func(ctx context.Context,request interface{}) (response interface{},err error)
type Endpoint func(context.Context, interface{}) (interface{}, error)

// MiddleWare 方便链式操作
type MiddleWare func(Endpoint) Endpoint

// HandlerFunc 错误处理
type HandlerFunc func(error) error

// Chain 连接成链路
// outer 最外层的
func Chain(outer MiddleWare, others ...MiddleWare) MiddleWare {
	return func(next Endpoint) Endpoint {
		for i := len(others) - 1; i >= 0; i-- { // reverse
			next = others[i](next)
		}
		return outer(next)
	}
}
