package trace

import (
	"github.com/songzhibin97/gkit/options"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type config struct {
	tracerProvider trace.TracerProvider
	propagator     propagation.TextMapPropagator
}

// WithPropagator with tracer propagator.
func WithPropagator(propagator propagation.TextMapPropagator) options.Option {
	return func(o interface{}) {
		o.(*config).propagator = propagator
	}
}

// WithTracerProvider configures the provider used by the returned Tracer. It
// does not install provider as OpenTelemetry's global provider.
func WithTracerProvider(provider trace.TracerProvider) options.Option {
	return func(o interface{}) {
		o.(*config).tracerProvider = provider
	}
}
