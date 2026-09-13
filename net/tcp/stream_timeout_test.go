package tcp

import (
	"errors"
	"io"
	"net"
	"os"
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

func TestStreamRetryWaitPreservesOverallTimeout(t *testing.T) {
	for _, exchange := range []bool{false, true} {
		for _, exhaust := range []bool{false, true} {
			name := "receive"
			if exchange {
				name = "send-receive"
			}
			if exhaust {
				name += "/exhausted"
			} else {
				name += "/idle"
			}
			t.Run(name, func(t *testing.T) {
				a, b := net.Pipe()
				defer a.Close()
				defer b.Close()
				c := NewConnByNetConn(a)
				c.SetRecvBufferInterval(5 * time.Millisecond)
				peerDone := make(chan error, 1)
				go func() {
					if exchange {
						request := make([]byte, len("request"))
						if _, err := io.ReadFull(b, request); err != nil {
							peerDone <- err
							return
						}
						if string(request) != "request" {
							peerDone <- errors.New("unexpected request payload")
							return
						}
					}
					_, err := b.Write([]byte("partial"))
					peerDone <- err
				}()
				total := time.Second
				interval := time.Millisecond
				if exhaust {
					total = 100 * time.Millisecond
					interval = time.Hour
				}
				retry := &Retry{Count: 1, Interval: interval}
				var data []byte
				var err error
				if exchange {
					data, err = c.SendRecvWithTimeout([]byte("request"), total, -1, retry)
				} else {
					data, err = c.RecvWithTimeout(-1, total, retry)
				}
				if peerErr := <-peerDone; peerErr != nil {
					t.Fatal(peerErr)
				}
				if string(data) != "partial" {
					t.Fatalf("data=%q want partial", data)
				}
				if retry.Count != 0 {
					t.Fatalf("retry count=%d want 0 after retry wait", retry.Count)
				}
				if exhaust {
					if !errors.Is(err, os.ErrDeadlineExceeded) {
						t.Fatalf("error=%v want exhausted total deadline", err)
					}
				} else if err != nil {
					t.Fatalf("idle with short retry wait error=%v", err)
				}
			})
		}
	}
}

type firstReadSignalConn struct {
	net.Conn
	firstRead chan struct{}
}

func (c *firstReadSignalConn) Read(p []byte) (int, error) {
	if c.firstRead != nil {
		close(c.firstRead)
		c.firstRead = nil
	}
	return c.Conn.Read(p)
}

func TestStreamRetryWaitErrorSurvivesExternalDeadlineExtension(t *testing.T) {
	for _, exchange := range []bool{false, true} {
		name := "receive"
		if exchange {
			name = "send-receive"
		}
		t.Run(name, func(t *testing.T) {
			a, b := net.Pipe()
			defer a.Close()
			defer b.Close()
			reading := make(chan struct{})
			c := NewConnByNetConn(&firstReadSignalConn{Conn: a, firstRead: reading})
			c.SetRecvBufferInterval(5 * time.Millisecond)
			retry := &Retry{Count: 1, Interval: time.Hour}
			done := make(chan sendRecvResult, 1)
			go func() {
				var data []byte
				var err error
				if exchange {
					data, err = c.SendRecvWithTimeout([]byte("request"), 100*time.Millisecond, -1, retry)
				} else {
					data, err = c.RecvWithTimeout(-1, 100*time.Millisecond, retry)
				}
				done <- sendRecvResult{data: data, err: err}
			}()
			if exchange {
				request := make([]byte, len("request"))
				if _, err := io.ReadFull(b, request); err != nil {
					t.Fatal(err)
				}
				if string(request) != "request" {
					t.Fatalf("request=%q", request)
				}
			}
			// The helper's original budget is installed before its first Read.
			// Replace the external deadline before delivering the first segment,
			// so the later idle probe snapshots the replacement's generation.
			select {
			case <-reading:
			case <-time.After(time.Second):
				t.Fatal("first Read not reached")
			}
			if err := c.SetReadDeadline(time.Now().Add(time.Hour)); err != nil {
				t.Fatal(err)
			}
			if _, err := b.Write([]byte("partial")); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-done:
				if string(got.data) != "partial" || !errors.Is(got.err, os.ErrDeadlineExceeded) {
					t.Fatalf("Recv=(%q,%v), want partial and original budget timeout", got.data, got.err)
				}
				if retry.Count != 0 {
					t.Fatalf("retry count=%d want consumed retry", retry.Count)
				}
			case <-time.After(time.Second):
				t.Fatal("retry wait did not honor its original budget")
			}
		})
	}
}
