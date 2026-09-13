package ip

import (
	"net"
	"net/http"
	"testing"
)

func TestClientPublicIPRejectsInvalidCandidates(t *testing.T) {
	for _, tt := range []struct{ name, forwarded, real, remote, want string }{
		{"invalid forwarded", "unknown", "", "127.0.0.1:1234", ""},
		{"forwarded fallback", "unknown", "8.8.8.8", "127.0.0.1:1234", "8.8.8.8"},
		{"real fallback", "", "not-an-ip", "8.8.4.4:1234", "8.8.4.4"},
		{"invalid remote", "", "", "unknown:1234", ""},
		{"invalid all", "unknown", "999.1.1.1", "unknown:1234", ""},
		{"forwarded public", " 8.8.8.8, 10.0.0.1", "1.1.1.1", "127.0.0.1:1234", "8.8.8.8"},
		{"public ipv6", "2001:4860:4860::8888", "", "127.0.0.1:1234", "2001:4860:4860::8888"},
		{"private fallback", "10.0.0.1", "192.168.1.1", "8.8.8.8:1234", "8.8.8.8"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{Header: make(http.Header), RemoteAddr: tt.remote}
			req.Header.Set(xForwardedFor, tt.forwarded)
			req.Header.Set(xRealIP, tt.real)
			got := ClientPublicIP(req)
			if got != tt.want {
				t.Fatalf("IP=%q want=%q", got, tt.want)
			}
			if got != "" && net.ParseIP(got) == nil {
				t.Fatalf("invalid IP=%q", got)
			}
		})
	}
}
