package trace

import (
	"context"
	"fmt"

	"github.com/songzhibin97/gkit/errors"
	"github.com/songzhibin97/gkit/options"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/protobuf/proto"
)

type Tracer struct {
	tracer trace.Tracer
	kind   trace.SpanKind
	opt    *config
}

// NewTracer 创建追踪器
func NewTracer(kind trace.SpanKind, opts ...options.Option) *Tracer {
	cfg := config{
		propagator: propagation.NewCompositeTextMapPropagator(Metadata{}, propagation.Baggage{}, propagation.TraceContext{}),
	}
	for _, o := range opts {
		o(&cfg)
	}
	if cfg.tracerProvider != nil {
		otel.SetTracerProvider(cfg.tracerProvider)
	}

	switch kind {
	case trace.SpanKindClient:
		return &Tracer{tracer: otel.Tracer("gkit"), kind: kind, opt: &cfg}
	case trace.SpanKindServer:
		return &Tracer{tracer: otel.Tracer("gkit"), kind: kind, opt: &cfg}
	default:
		panic(fmt.Sprintf("unsupported span kind: %v", kind))
	}
}

// Start 开始追踪
func (t *Tracer) Start(ctx context.Context, operation string, carrier propagation.TextMapCarrier) (context.Context, trace.Span) {
	if t.kind == trace.SpanKindServer {
		ctx = t.opt.propagator.Extract(ctx, carrier)
	}
	ctx, span := t.tracer.Start(ctx,
		operation,
		trace.WithSpanKind(t.kind),
	)
	if t.kind == trace.SpanKindClient {
		t.opt.propagator.Inject(ctx, carrier)
	}
	return ctx, span
}

// End 完成追踪
func (t *Tracer) End(ctx context.Context, span trace.Span, m interface{}, err error) {
	if err != nil {
		span.RecordError(err)
		if e := errors.FromError(err); e != nil {
			span.SetAttributes(attribute.Key("rpc.status_code").Int64(int64(e.Code)))
		}
		span.SetStatus(codes.Error, err.Error())
	} else {
		span.SetStatus(codes.Ok, "OK")
	}

	if p, ok := m.(proto.Message); ok {
		if t.kind == trace.SpanKindServer {
			span.SetAttributes(attribute.Key("send_msg.size").Int(proto.Size(p)))
		} else {
			span.SetAttributes(attribute.Key("recv_msg.size").Int(proto.Size(p)))
		}
	}
	span.End()
}
