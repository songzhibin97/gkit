package trace

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	otelemetrytrace "go.opentelemetry.io/otel/trace"
)

func TestNewTracerWithProviderDoesNotReplaceGlobalProvider(t *testing.T) {
	originalProvider := otel.GetTracerProvider()
	localProvider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(tracetest.NewSpanRecorder()))
	t.Cleanup(func() {
		if err := localProvider.Shutdown(context.Background()); err != nil {
			t.Errorf("shutdown local provider: %v", err)
		}
	})

	tracer := NewTracer(otelemetrytrace.SpanKindClient, WithTracerProvider(localProvider))
	if got := otel.GetTracerProvider(); got != originalProvider {
		t.Fatalf("NewTracer replaced global provider: got %p, want original %p", got, originalProvider)
	}

	_, localSpan := tracer.Start(context.Background(), "local", propagation.MapCarrier{})
	if got := localSpan.TracerProvider(); got != localProvider {
		t.Fatalf("configured tracer provider = %p, want local provider %p", got, localProvider)
	}
	localSpan.End()

	globalTracer := NewTracer(otelemetrytrace.SpanKindClient)
	_, globalSpan := globalTracer.Start(context.Background(), "global", propagation.MapCarrier{})
	if got := globalSpan.TracerProvider(); got != originalProvider {
		t.Fatalf("default tracer provider = %p, want original global %p", got, originalProvider)
	}
	globalSpan.End()
}
