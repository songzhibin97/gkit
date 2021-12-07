package trace

import (
	"context"

	"github.com/songzhibin97/gkit/middleware"
	"github.com/songzhibin97/gkit/options"
	"go.opentelemetry.io/otel/trace"
)

// WithServer returns a new server middleware for OpenTelemetry.
func WithServer(opts ...options.Option) middleware.MiddleWare {
	tracer := NewTracer(trace.SpanKindServer, opts...)
	return func(handler middleware.Endpoint) middleware.Endpoint {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if tr, ok := FromServerTransportContext(ctx); ok {
				var span trace.Span
				ctx, span = tracer.Start(ctx, tr.Operation(), tr.RequestHeader())
				setServerSpan(ctx, span, req)
				defer func() { tracer.End(ctx, span, reply, err) }()
			}
			return handler(ctx, req)
		}
	}
}

// WithClient returns a new client middleware for OpenTelemetry.
func WithClient(opts ...options.Option) middleware.MiddleWare {
	tracer := NewTracer(trace.SpanKindClient, opts...)
	return func(handler middleware.Endpoint) middleware.Endpoint {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			if tr, ok := FromClientTransportContext(ctx); ok {
				var span trace.Span
				ctx, span = tracer.Start(ctx, tr.Operation(), tr.RequestHeader())
				setClientSpan(ctx, span, req)
				defer func() { tracer.End(ctx, span, reply, err) }()
			}
			return handler(ctx, req)
		}
	}
}
