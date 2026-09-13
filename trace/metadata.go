package trace

import (
	"context"

	"github.com/songzhibin97/gkit/internal/metadata"
	"go.opentelemetry.io/otel/propagation"
)

const serverMark = "x-meta-service-name"

type Metadata struct {
	Name string
}

var _ propagation.TextMapPropagator = Metadata{}

// Inject set cross-cutting concerns from the Context into the carrier.
func (m Metadata) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
	carrier.Set(serverMark, m.Name)
}

// Extract reads cross-cutting concerns from the carrier into a Context.
func (m Metadata) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
	name := carrier.Get(serverMark)
	if name != "" {
		md, _ := metadata.FromServerContext(ctx)
		md = md.Clone()
		md.Set(serverMark, name)
		ctx = metadata.NewServerContext(ctx, md)
	}
	return ctx
}

// Fields returns the keys who's values are set with Inject.
func (m Metadata) Fields() []string {
	return []string{serverMark}
}
