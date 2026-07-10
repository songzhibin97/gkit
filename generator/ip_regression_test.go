package generator

import (
	"net"
	"testing"
)

func TestIpToUint16NormalizesParsedIPv4(t *testing.T) {
	got, err := IpToUint16(net.ParseIP("192.168.2.3"))
	if err != nil {
		t.Fatal(err)
	}
	if got != 515 {
		t.Fatalf("IpToUint16(192.168.2.3) = %d, want 515", got)
	}
}

func TestIpToUint16RejectsInvalidInput(t *testing.T) {
	tests := []struct {
		name string
		ip   net.IP
	}{
		{name: "nil", ip: nil},
		{name: "short", ip: net.IP{1, 2, 3}},
		{name: "IPv6", ip: net.ParseIP("2001:db8::1")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err, panicked := callIpToUint16(tt.ip)
			if panicked {
				t.Fatalf("IpToUint16(%v) panicked", tt.ip)
			}
			if err == nil {
				t.Fatalf("IpToUint16(%v) returned nil error", tt.ip)
			}
		})
	}
}

func callIpToUint16(ip net.IP) (value uint16, err error, panicked bool) {
	defer func() {
		if recover() != nil {
			panicked = true
		}
	}()
	value, err = IpToUint16(ip)
	return value, err, false
}
