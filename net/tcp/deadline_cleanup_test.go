package tcp

import (
	"errors"
	"net"
	"testing"
	"time"
)

type cleanupFailureConn struct {
	net.Conn
	cleanupErr error
}

func (c *cleanupFailureConn) SetReadDeadline(d time.Time) error {
	if d.IsZero() {
		return c.cleanupErr
	}
	return c.Conn.SetReadDeadline(d)
}
func (c *cleanupFailureConn) SetWriteDeadline(d time.Time) error {
	if d.IsZero() {
		return c.cleanupErr
	}
	return c.Conn.SetWriteDeadline(d)
}

func TestTimeoutReportsDeadlineCleanupFailure(t *testing.T) {
	for _, send := range []bool{false, true} {
		for _, failIO := range []bool{false, true} {
			name := "recv"
			if send {
				name = "send"
			}
			if failIO {
				name += "-timeout"
			}
			t.Run(name, func(t *testing.T) {
				a, b := net.Pipe()
				defer a.Close()
				defer b.Close()
				sentinel := errors.New("deadline cleanup sentinel")
				c := NewConnByNetConn(&cleanupFailureConn{a, sentinel})
				peerDone := make(chan error, 1)
				if !failIO {
					go func() {
						var err error
						if send {
							p := make([]byte, 1)
							_, err = b.Read(p)
							if err == nil && string(p) != "x" {
								err = errors.New("unexpected peer payload")
							}
						} else {
							_, err = b.Write([]byte("x"))
						}
						peerDone <- err
					}()
				}
				var err error
				if send {
					err = c.SendWithTimeout([]byte("x"), 30*time.Millisecond, nil)
				} else {
					var data []byte
					data, err = c.RecvWithTimeout(1, 30*time.Millisecond, nil)
					if !failIO && string(data) != "x" {
						t.Fatalf("data=%q", data)
					}
				}
				if !failIO {
					if e := <-peerDone; e != nil {
						t.Fatal(e)
					}
				}
				if !errors.Is(err, sentinel) {
					t.Fatalf("error=%v, want cleanup sentinel", err)
				}
				if failIO {
					var ne net.Error
					if !errors.As(err, &ne) || !ne.Timeout() {
						t.Fatalf("error=%v, lost I/O timeout", err)
					}
				}
			})
		}
	}
}
