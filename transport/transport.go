package transport

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

	// Endpoint 注册点
	Endpoint interface {
		Endpoint() (*url.URL, error)
	}

	// Transport 链路追踪的上下文
	Transport struct {
		Kind     Kind
		Endpoint string
	}
)

// transportKey 保证context.WithValue唯一性
type transportKey struct{}

// NewTransportContext 新建链路上下文
func NewTransportContext(ctx context.Context, tp Transport) context.Context {
	return context.WithValue(ctx, transportKey{}, tp)
}

// FromTransportContext 通过context获取链路信息
func FromTransportContext(ctx context.Context) (Transport, bool) {
	v, ok := ctx.Value(transportKey{}).(Transport)
	return v, ok
}
