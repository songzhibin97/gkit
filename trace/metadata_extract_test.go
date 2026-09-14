package trace

import (
	"context"
	"testing"

	"github.com/songzhibin97/gkit/internal/metadata"
	"go.opentelemetry.io/otel/propagation"
)

func TestMetadataExtractPreservesParentAndSibling(t *testing.T) {
	base := metadata.NewMetadata(map[string]string{serverMark: "original", "keep": "retained"})
	parent := metadata.NewServerContext(context.Background(), base)
	sibling, cancel := context.WithCancel(parent)
	defer cancel()
	first := (Metadata{}).Extract(parent, propagation.MapCarrier{serverMark: "first"})
	second := (Metadata{}).Extract(parent, propagation.MapCarrier{serverMark: "second"})
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		service string
	}{
		{"parent", parent, "original"}, {"earlier sibling", sibling, "original"}, {"first child", first, "first"}, {"second child", second, "second"},
	} {
		md, ok := metadata.FromServerContext(tc.ctx)
		if !ok {
			t.Fatalf("%s metadata missing", tc.name)
		}
		if got := md.GetValue(serverMark); got != tc.service {
			t.Errorf("%s service = %q, want %q", tc.name, got, tc.service)
		}
		if got := md.GetValue("keep"); got != "retained" {
			t.Errorf("%s lost existing metadata", tc.name)
		}
	}
	childMD, _ := metadata.FromServerContext(first)
	childMD.Set("child-only", "value")
	for _, ctx := range []context.Context{parent, sibling, second} {
		md, _ := metadata.FromServerContext(ctx)
		if md.GetValue("child-only") != "" {
			t.Error("child metadata still aliases parent or sibling")
		}
	}
}

func TestMetadataExtractCreatesMetadata(t *testing.T) {
	for _, tc := range []struct {
		name string
		ctx  context.Context
	}{
		{"absent", context.Background()},
		{"nil metadata", metadata.NewServerContext(context.Background(), nil)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			child := (Metadata{}).Extract(tc.ctx, propagation.MapCarrier{serverMark: "incoming"})
			md, ok := metadata.FromServerContext(child)
			if !ok || md.GetValue(serverMark) != "incoming" {
				t.Fatalf("child metadata = %v, present=%t", md, ok)
			}
			if md, _ := metadata.FromServerContext(tc.ctx); len(md) != 0 {
				t.Fatalf("parent changed: %v", md)
			}
		})
	}
}

func TestMetadataExtractEmptyCarrierPreservesContext(t *testing.T) {
	parent := metadata.NewServerContext(context.Background(), metadata.NewMetadata(map[string]string{serverMark: "original"}))
	for _, ctx := range []context.Context{context.Background(), parent} {
		for _, carrier := range []propagation.MapCarrier{{}, {serverMark: ""}} {
			if got := (Metadata{}).Extract(ctx, carrier); got != ctx {
				t.Fatal("empty carrier replaced the context")
			}
		}
	}
	md, _ := metadata.FromServerContext(parent)
	if md.GetValue(serverMark) != "original" {
		t.Fatal("empty carrier changed existing service metadata")
	}
}
