package trace

import "testing"

func TestParseTargetHandlesSchemeAndBareEndpoints(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		want     string
	}{
		{name: "scheme", endpoint: "grpc://127.0.0.1:9000", want: "127.0.0.1:9000"},
		{name: "bare", endpoint: "127.0.0.1:9000", want: "127.0.0.1:9000"},
		{name: "unix path", endpoint: "unix:///tmp/gkit.sock", want: "tmp/gkit.sock"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTarget(tt.endpoint)
			if err != nil {
				t.Fatalf("parseTarget(%q): %v", tt.endpoint, err)
			}
			if got != tt.want {
				t.Fatalf("parseTarget(%q) = %q, want %q", tt.endpoint, got, tt.want)
			}
		})
	}
}

func TestSchemeTargetProducesPeerAttributes(t *testing.T) {
	address, err := parseTarget("grpc://127.0.0.1:9000")
	if err != nil {
		t.Fatal(err)
	}
	attrs := peerAttr(address)
	if len(attrs) != 2 {
		t.Fatalf("peerAttr(%q) returned %d attributes, want 2", address, len(attrs))
	}
	if got := attrs[0].Value.AsString(); got != "127.0.0.1" {
		t.Fatalf("peer IP = %q, want 127.0.0.1", got)
	}
	if got := attrs[1].Value.AsString(); got != "9000" {
		t.Fatalf("peer port = %q, want 9000", got)
	}
}
