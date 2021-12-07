package trace

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel/propagation"
	sdk "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

var _ Transporter = &Transport{}

type _Transport struct {
	kind      Kind
	endpoint  string
	operation string
	header    headerCarrier
}

func (tr *_Transport) Kind() Kind             { return tr.kind }
func (tr *_Transport) Endpoint() string       { return tr.endpoint }
func (tr *_Transport) Operation() string      { return tr.operation }
func (tr *_Transport) RequestHeader() Header  { return tr.header }
func (tr *_Transport) ResponseHeader() Header { return tr.header }

func TestTrace(t *testing.T) {
	carrier := headerCarrier{}
	tp := sdk.NewTracerProvider(sdk.WithSampler(sdk.TraceIDRatioBased(0)))

	// caller use Inject
	tracer := NewTracer(trace.SpanKindClient, WithTracerProvider(tp), WithPropagator(propagation.NewCompositeTextMapPropagator(propagation.Baggage{}, propagation.TraceContext{})))
	ts := &_Transport{kind: KindHTTP, header: carrier}

	ctx, aboveSpan := tracer.Start(NewClientTransportContext(context.Background(), ts), ts.Operation(), ts.RequestHeader())
	defer tracer.End(ctx, aboveSpan, nil, nil)

	// server use Extract fetch traceInfo from carrier
	tracer = NewTracer(trace.SpanKindServer, WithPropagator(propagation.NewCompositeTextMapPropagator(propagation.Baggage{}, propagation.TraceContext{})))
	ts = &_Transport{kind: KindHTTP, header: carrier}

	ctx, span := tracer.Start(NewServerTransportContext(ctx, ts), ts.Operation(), ts.RequestHeader())
	defer tracer.End(ctx, span, nil, nil)

	if aboveSpan.SpanContext().TraceID() != span.SpanContext().TraceID() {
		t.Fatalf("TraceID failed to deliver")
	}

	if v, ok := FromClientTransportContext(ctx); !ok || len(v.RequestHeader().Keys()) == 0 {
		t.Fatalf("traceHeader failed to deliver")
	} else {
		t.Log(v)
		t.Log(v.RequestHeader().Keys())
	}
}
