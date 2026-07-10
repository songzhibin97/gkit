package ip

import (
	"net/http"
	"testing"
)

func TestHasLocalIPAddrIPv6(t *testing.T) {
	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{name: "loopback", ip: "::1", want: true},
		{name: "ula fc", ip: "fc00::1", want: true},
		{name: "ula fd", ip: "fd12:3456:789a::1", want: true},
		{name: "link local unicast", ip: "fe80::1", want: true},
		{name: "link local multicast", ip: "ff02::1", want: true},
		{name: "global", ip: "2001:4860:4860::8888", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasLocalIPAddr(tt.ip); got != tt.want {
				t.Fatalf("HasLocalIPAddr(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestClientPublicIPSkipsLocalIPv6(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "http://example.test", nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	req.Header.Set(xForwardedFor, "fd12:3456:789a::1")
	req.Header.Set(xRealIP, "2001:4860:4860::8888")
	req.RemoteAddr = "[fe80::1]:443"

	if got := ClientPublicIP(req); got != "2001:4860:4860::8888" {
		t.Fatalf("ClientPublicIP() = %q, want the first non-local IPv6 address", got)
	}
}
