package trace

import (
	"context"
	"net/url"
)

// package transport 链路追踪

const (
	KindGRPC Kind = "GRPC"
	KindHTTP Kind = "HTTP"
)

type (
	Kind string

	// Server 链路追踪的服务
	Server interface {
		Start(ctx context.Context) error
		Shutdown(ctx context.Context) error
	}

	// EndPointer 注册点
	EndPointer interface {
		Endpoint() (*url.URL, error)
	}

	// Header 抽象获取协议头部的动作行为
	Header interface {
		Get(key string) string
		Set(key string, value string)
		Keys() []string
	}

	// Transporter 链路追踪的上下文
	Transporter interface {
		// Kind 返回 KindGRPC or KindHTTP 用于区分协议调用
		Kind() Kind

		// Endpoint
		// Server Transporter: grpc://127.0.0.1:9000
		// Client Transporter: discovery://provider-demo
		Endpoint() string

		// Operation protobuf 行为
		Operation() string

		// RequestHeader 返回请求头
		RequestHeader() Header

		// ResponseHeader 返回响应头
		ResponseHeader() Header
	}
)

func (k Kind) String() string { return string(k) }

// transportKey 保证context.WithValue唯一性
type serverTransportKey struct{}
type clientTransportKey struct{}

// NewServerTransportContext 新建服务端链路上下文
func NewServerTransportContext(ctx context.Context, tp Transporter) context.Context {
	return context.WithValue(ctx, serverTransportKey{}, tp)
}

// FromServerTransportContext 通过服务端context获取链路信息
func FromServerTransportContext(ctx context.Context) (Transporter, bool) {
	v, ok := ctx.Value(serverTransportKey{}).(Transporter)
	return v, ok
}

// NewClientTransportContext 新建客户端链路上下文
func NewClientTransportContext(ctx context.Context, tp Transporter) context.Context {
	return context.WithValue(ctx, clientTransportKey{}, tp)
}

// FromClientTransportContext 通过客户端context获取链路信息
func FromClientTransportContext(ctx context.Context) (Transporter, bool) {
	v, ok := ctx.Value(clientTransportKey{}).(Transporter)
	return v, ok
}
