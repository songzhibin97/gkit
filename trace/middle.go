package trace

import (
	"context"
	"errors"

	"github.com/songzhibin97/gkit/middleware"
	"github.com/songzhibin97/gkit/options"
	"go.opentelemetry.io/otel/trace"
)

// WithServer returns a new server middleware for OpenTelemetry.
func WithServer(opts ...options.Option) middleware.MiddleWare {
	tracer := NewTracer(trace.SpanKindServer, opts...)
	return func(handler middleware.Endpoint) middleware.Endpoint {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			completed := false
			if tr, ok := FromServerTransportContext(ctx); ok {
				var span trace.Span
				ctx, span = tracer.Start(ctx, tr.Operation(), tr.RequestHeader())
				setServerSpan(ctx, span, req)
				defer func() {
					if !completed {
						// Do not recover or expose panic contents; this also handles panic(nil).
						err = errors.New("handler panicked")
					}
					tracer.End(ctx, span, reply, err)
				}()
			}
			reply, err = handler(ctx, req)
			completed = true
			return reply, err
		}
	}
}

// WithClient returns a new client middleware for OpenTelemetry.
func WithClient(opts ...options.Option) middleware.MiddleWare {
	tracer := NewTracer(trace.SpanKindClient, opts...)
	return func(handler middleware.Endpoint) middleware.Endpoint {
		return func(ctx context.Context, req interface{}) (reply interface{}, err error) {
			completed := false
			if tr, ok := FromClientTransportContext(ctx); ok {
				var span trace.Span
				ctx, span = tracer.Start(ctx, tr.Operation(), tr.RequestHeader())
				setClientSpan(ctx, span, req)
				defer func() {
					if !completed {
						// Do not recover or expose panic contents; this also handles panic(nil).
						err = errors.New("handler panicked")
					}
					tracer.End(ctx, span, reply, err)
				}()
			}
			reply, err = handler(ctx, req)
			completed = true
			return reply, err
		}
	}
}
