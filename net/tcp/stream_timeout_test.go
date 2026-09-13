package tcp

import (
	"errors"
	"net"
	"testing"
	"time"
)

func TestStreamDistinguishesIdleFromOverallTimeout(t *testing.T) {
	for _, tt := range []struct {
		name        string
		idle, total time.Duration
		wantTimeout bool
	}{
		{"overall", time.Hour, 30 * time.Millisecond, true},
		{"idle", 5 * time.Millisecond, time.Second, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			c := NewConnByNetConn(a)
			c.SetRecvBufferInterval(tt.idle)
			done := make(chan error, 1)
			go func() { _, err := b.Write([]byte("partial")); done <- err }()
			data, err := c.RecvWithTimeout(-1, tt.total, nil)
			if peerErr := <-done; peerErr != nil {
				t.Fatal(peerErr)
			}
			if string(data) != "partial" {
				t.Fatalf("data=%q", data)
			}
			if tt.wantTimeout {
				var ne net.Error
				if !errors.As(err, &ne) || !ne.Timeout() {
					t.Fatalf("error=%v, want overall timeout", err)
				}
			} else if err != nil {
				t.Fatalf("idle error=%v", err)
			}
			// A successful fresh exchange proves the temporary and total deadlines cleared.
			go func() { _, err := b.Write([]byte("x")); done <- err }()
			data, err = c.Recv(1, nil)
			if err != nil || string(data) != "x" {
				t.Fatalf("reuse=(%q,%v)", data, err)
			}
			if peerErr := <-done; peerErr != nil {
				t.Fatal(peerErr)
			}
		})
	}
}

type earlyTimeoutConn struct {
	net.Conn
	reads int
}

func (c *earlyTimeoutConn) Read(p []byte) (int, error) {
	c.reads++
	if c.reads == 2 {
		return 0, issue83TimeoutError{}
	}
	return c.Conn.Read(p)
}

func TestStreamPreservesTimeoutBeforeIdleDeadline(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	c := NewConnByNetConn(&earlyTimeoutConn{Conn: a})
	c.SetRecvBufferInterval(time.Hour)
	done := make(chan error, 1)
	go func() { _, err := b.Write([]byte("partial")); done <- err }()
	data, err := c.Recv(-1, nil)
	if peerErr := <-done; peerErr != nil {
		t.Fatal(peerErr)
	}
	var ne net.Error
	if string(data) != "partial" || !errors.As(err, &ne) || !ne.Timeout() {
		t.Fatalf("Recv=(%q,%v), want partial and early timeout", data, err)
	}
}
