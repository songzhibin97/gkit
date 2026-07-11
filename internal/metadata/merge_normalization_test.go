package metadata

import (
	"context"
	"testing"
)

func TestMergeToClientContextNormalizesKeys(t *testing.T) {
	ctx := MergeToClientContext(context.Background(), Metadata{
		"X-Request-ID": "request-1",
	})
	md, ok := FromClientContext(ctx)
	if !ok {
		t.Fatal("merged metadata is missing from client context")
	}

	for _, key := range []string{"x-request-id", "X-REQUEST-ID", "X-Request-Id"} {
		if got := md.GetValue(key); got != "request-1" {
			t.Fatalf("GetValue(%q) = %q, want %q", key, got, "request-1")
		}
	}
	if _, exists := md["X-Request-ID"]; exists {
		t.Fatal("merged metadata retained a non-normalized key")
	}
}

func TestMergeToClientContextOverwritesExistingNormalizedKey(t *testing.T) {
	base := NewMetadata(map[string]string{"x-request-id": "old"})
	ctx := NewClientContext(context.Background(), base)
	ctx = MergeToClientContext(ctx, Metadata{"X-Request-ID": "new"})

	md, ok := FromClientContext(ctx)
	if !ok {
		t.Fatal("merged metadata is missing from client context")
	}
	if got := md.GetValue("x-request-id"); got != "new" {
		t.Fatalf("GetValue() = %q, want merged value %q", got, "new")
	}
	if len(md) != 1 {
		t.Fatalf("merged metadata has %d entries, want one normalized key", len(md))
	}
}

func TestMergeToClientContextIgnoresEmptyKeyOrValue(t *testing.T) {
	ctx := MergeToClientContext(context.Background(), Metadata{
		"":            "value",
		"EMPTY-VALUE": "",
	})
	md, ok := FromClientContext(ctx)
	if !ok {
		t.Fatal("merged metadata is missing from client context")
	}

	if len(md) != 0 {
		t.Fatalf("merged metadata = %#v, want empty key and value ignored", md)
	}
}
