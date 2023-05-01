package ip

import (
	"errors"
	"math"
	"net"
	"net/http"
	"strings"
)

const (
	xForwardedFor = "X-Forwarded-For"
	xRealIP       = "X-Real-IP"
)

func HasLocalIPAddr(ip string) bool {
	return HasLocalIP(net.ParseIP(ip))
}

func HasLocalIP(ip net.IP) bool {
	if ip.IsLoopback() {
		return true
	}

	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	return ip4[0] == 10 || // 10.0.0.0/8
		(ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31) || // 172.16.0.0/12
		(ip4[0] == 169 && ip4[1] == 254) || // 169.254.0.0/16
		(ip4[0] == 192 && ip4[1] == 168) // 192.168.0.0/16
}

func ClientIP(req *http.Request) string {
	if ip := strings.TrimSpace(strings.Split(req.Header.Get(xForwardedFor), ",")[0]); ip != "" {
		return ip
	}
	if ip := strings.TrimSpace(req.Header.Get(xRealIP)); ip != "" {
		return ip
	}
	return RemoteIP(req)
}

func ClientPublicIP(req *http.Request) string {
	if ip := strings.TrimSpace(strings.Split(req.Header.Get(xForwardedFor), ",")[0]); ip != "" && !HasLocalIPAddr(ip) {
		return ip
	}
	if ip := strings.TrimSpace(req.Header.Get(xRealIP)); ip != "" && !HasLocalIPAddr(ip) {
		return ip
	}
	if ip := RemoteIP(req); ip != "" && !HasLocalIPAddr(ip) {
		return ip
	}
	return ""
}

func RemoteIP(req *http.Request) string {
	ip, _, err := net.SplitHostPort(strings.TrimSpace(req.RemoteAddr))
	if err != nil {
		return ""
	}
	return ip
}

func IsWebsocket(req *http.Request) bool {
	if strings.Contains(strings.ToLower(requestHeader(req, "Connection")), "upgrade") &&
		strings.EqualFold(requestHeader(req, "Upgrade"), "websocket") {
		return true
	}
	return false
}

func requestHeader(req *http.Request, key string) string {
	return req.Header.Get(key)
}

// ContentType returns the Content-Type header of the request.
func ContentType(req *http.Request) string {
	return filterFlags(requestHeader(req, "Content-Type"))
}

func filterFlags(content string) string {
	for i, char := range content {
		if char == ' ' || char == ';' {
			return content[:i]
		}
	}
	return content
}

func StringToLong(ip string) (uint, error) {
	b := net.ParseIP(ip).To4()
	if b == nil {
		return 0, errors.New("invalid ipv4 format")
	}

	return uint(b[3]) | uint(b[2])<<8 | uint(b[1])<<16 | uint(b[0])<<24, nil
}

func LongToIPString(i uint) (string, error) {
	if i > math.MaxUint32 {
		return "", errors.New("beyond the scope of ipv4")
	}

	ip := make(net.IP, net.IPv4len)
	ip[0] = byte(i >> 24)
	ip[1] = byte(i >> 16)
	ip[2] = byte(i >> 8)
	ip[3] = byte(i)

	return ip.String(), nil
}

func ToLong(ip net.IP) (uint, error) {
	b := ip.To4()
	if b == nil {
		return 0, errors.New("invalid ipv4 format")
	}

	return uint(b[3]) | uint(b[2])<<8 | uint(b[1])<<16 | uint(b[0])<<24, nil
}

func LongToIP(i uint) (net.IP, error) {
	if i > math.MaxUint32 {
		return nil, errors.New("beyond the scope of ipv4")
	}

	ip := make(net.IP, net.IPv4len)
	ip[0] = byte(i >> 24)
	ip[1] = byte(i >> 16)
	ip[2] = byte(i >> 8)
	ip[3] = byte(i)

	return ip, nil
}
