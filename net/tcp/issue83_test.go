package tcp

import (
	"errors"
	"io"
	"net"
	"reflect"
	"sync"
	"testing"
	"time"
)

type issue83ReadStep struct {
	data []byte
	err  error
}

type issue83ScriptedConn struct {
	mu            sync.Mutex
	reads         []issue83ReadStep
	readIndex     int
	write         func([]byte) (int, error)
	readDeadlines []time.Time
}

func (c *issue83ScriptedConn) Read(p []byte) (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.readIndex >= len(c.reads) {
		return 0, io.EOF
	}
	step := c.reads[c.readIndex]
	c.readIndex++
	return copy(p, step.data), step.err
}

func (c *issue83ScriptedConn) Write(p []byte) (int, error) {
	if c.write == nil {
		return len(p), nil
	}
	return c.write(p)
}

func (*issue83ScriptedConn) Close() error                     { return nil }
func (*issue83ScriptedConn) LocalAddr() net.Addr              { return issue83Addr("local") }
func (*issue83ScriptedConn) RemoteAddr() net.Addr             { return issue83Addr("remote") }
func (c *issue83ScriptedConn) SetDeadline(t time.Time) error  { return c.SetReadDeadline(t) }
func (*issue83ScriptedConn) SetWriteDeadline(time.Time) error { return nil }
func (c *issue83ScriptedConn) SetReadDeadline(t time.Time) error {
	c.mu.Lock()
	c.readDeadlines = append(c.readDeadlines, t)
	c.mu.Unlock()
	return nil
}

type issue83Addr string

func (a issue83Addr) Network() string { return string(a) }
func (a issue83Addr) String() string  { return string(a) }

type issue83TimeoutError struct{}

func (issue83TimeoutError) Error() string   { return "read timeout" }
func (issue83TimeoutError) Timeout() bool   { return true }
func (issue83TimeoutError) Temporary() bool { return true }

// Behavior 1: a line terminated by EOF returns its bytes and the EOF; an
// empty EOF must not index an empty slice.
func TestIssue83RecvLinePreservesPartialEOF(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want []byte
	}{
		{name: "partial", data: []byte("partial"), want: []byte("partial")},
		{name: "empty", want: []byte{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reads := make([]issue83ReadStep, 0, 2)
			if len(tt.data) > 0 {
				reads = append(reads, issue83ReadStep{data: tt.data})
			}
			reads = append(reads, issue83ReadStep{err: io.EOF})
			raw := &issue83ScriptedConn{reads: reads}
			conn := NewConnByNetConn(raw)
			var got []byte
			var err error
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("RecvLine panicked at EOF: %v", recovered)
					}
				}()
				got, err = conn.RecvLine(nil)
			}()
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("RecvLine() data = %q, want %q", got, tt.want)
			}
			if !errors.Is(err, io.EOF) {
				t.Fatalf("RecvLine() error = %v, want EOF", err)
			}
		})
	}
}

// Behavior 2: successful short writes continue from the first unwritten byte;
// a retry after a partial error also receives only the remaining suffix.
func TestIssue83SendTracksPartialWrites(t *testing.T) {
	t.Run("short write without error", func(t *testing.T) {
		var written []byte
		raw := &issue83ScriptedConn{}
		raw.write = func(p []byte) (int, error) {
			n := 2
			if len(p) < n {
				n = len(p)
			}
			written = append(written, p[:n]...)
			return n, nil
		}
		if err := NewConnByNetConn(raw).Send([]byte("abcdef"), nil); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		if string(written) != "abcdef" {
			t.Fatalf("written bytes = %q, want %q", written, "abcdef")
		}
	})

	t.Run("partial error retries suffix", func(t *testing.T) {
		transient := errors.New("transient write failure")
		var calls [][]byte
		raw := &issue83ScriptedConn{}
		raw.write = func(p []byte) (int, error) {
			calls = append(calls, append([]byte(nil), p...))
			if len(calls) == 1 {
				return 2, transient
			}
			return len(p), nil
		}
		retry := &Retry{Count: 1, Interval: time.Nanosecond}
		if err := NewConnByNetConn(raw).Send([]byte("abcdef"), retry); err != nil {
			t.Fatalf("Send() error = %v", err)
		}
		want := [][]byte{[]byte("abcdef"), []byte("cdef")}
		if !reflect.DeepEqual(calls, want) {
			t.Fatalf("Write calls = %q, want %q", calls, want)
		}
	})
}

func TestIssue83SendReturnsFinalErrors(t *testing.T) {
	t.Run("EOF", func(t *testing.T) {
		raw := &issue83ScriptedConn{write: func([]byte) (int, error) {
			return 0, io.EOF
		}}
		err := NewConnByNetConn(raw).Send([]byte("x"), nil)
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Send() error = %v, want EOF", err)
		}
	})

	t.Run("error after all bytes", func(t *testing.T) {
		writeErr := errors.New("write completed with terminal error")
		raw := &issue83ScriptedConn{write: func(p []byte) (int, error) {
			return len(p), writeErr
		}}
		err := NewConnByNetConn(raw).Send([]byte("done"), &Retry{Count: 1, Interval: time.Nanosecond})
		if !errors.Is(err, writeErr) {
			t.Fatalf("Send() error = %v, want %v", err, writeErr)
		}
	})

	t.Run("zero progress", func(t *testing.T) {
		raw := &issue83ScriptedConn{write: func([]byte) (int, error) {
			return 0, nil
		}}
		err := NewConnByNetConn(raw).Send([]byte("x"), nil)
		if !errors.Is(err, io.ErrNoProgress) {
			t.Fatalf("Send() error = %v, want io.ErrNoProgress", err)
		}
	})
}

// Behavior 3: fixed reads surface premature EOF with their partial data.
func TestIssue83RecvFixedLengthReportsEarlyEOF(t *testing.T) {
	t.Run("partial", func(t *testing.T) {
		raw := &issue83ScriptedConn{reads: []issue83ReadStep{
			{data: []byte("abc")},
			{err: io.EOF},
		}}
		got, err := NewConnByNetConn(raw).Recv(5, nil)
		if string(got) != "abc" {
			t.Fatalf("Recv(5) data = %q, want %q", got, "abc")
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) {
			t.Fatalf("Recv(5) error = %v, want io.ErrUnexpectedEOF", err)
		}
	})

	t.Run("empty", func(t *testing.T) {
		got, err := NewConnByNetConn(&issue83ScriptedConn{reads: []issue83ReadStep{{err: io.EOF}}}).Recv(5, nil)
		if len(got) != 0 {
			t.Fatalf("Recv(5) data = %q, want empty", got)
		}
		if !errors.Is(err, io.EOF) {
			t.Fatalf("Recv(5) error = %v, want EOF", err)
		}
	})
}

// Behavior 3: streaming reads drain all available segments and restore the
// deadline that was active before their idle probe.
func TestIssue83RecvStreamDrainsAndRestoresDeadline(t *testing.T) {
	raw := &issue83ScriptedConn{reads: []issue83ReadStep{
		{data: []byte("first-")},
		{data: []byte("second")},
		{err: issue83TimeoutError{}},
	}}
	conn := NewConnByNetConn(raw)
	conn.SetRecvBufferInterval(20 * time.Millisecond)
	original := time.Now().Add(time.Minute).Round(0)
	if err := conn.SetReadDeadline(original); err != nil {
		t.Fatalf("SetReadDeadline() error = %v", err)
	}

	got, err := conn.Recv(-1, nil)
	if err != nil {
		t.Fatalf("Recv(-1) error = %v", err)
	}
	if string(got) != "first-second" {
		t.Fatalf("Recv(-1) data = %q, want %q", got, "first-second")
	}
	raw.mu.Lock()
	deadlines := append([]time.Time(nil), raw.readDeadlines...)
	raw.mu.Unlock()
	if len(deadlines) < 3 {
		t.Fatalf("read deadline calls = %v, want idle probes plus restoration", deadlines)
	}
	if !deadlines[len(deadlines)-1].Equal(original) {
		t.Fatalf("final read deadline = %v, want restored %v", deadlines[len(deadlines)-1], original)
	}
	for _, deadline := range deadlines[1 : len(deadlines)-1] {
		if deadline.Equal(original) {
			t.Fatalf("idle deadline %v unexpectedly equals original deadline", deadline)
		}
	}
}

func TestIssue83RecvStreamDrainsUntilEOF(t *testing.T) {
	raw := &issue83ScriptedConn{reads: []issue83ReadStep{
		{data: []byte("one-")},
		{data: []byte("two")},
		{err: io.EOF},
	}}
	got, err := NewConnByNetConn(raw).Recv(-1, nil)
	if err != nil {
		t.Fatalf("Recv(-1) error = %v", err)
	}
	if string(got) != "one-two" {
		t.Fatalf("Recv(-1) data = %q, want %q", got, "one-two")
	}
	raw.mu.Lock()
	deadlines := append([]time.Time(nil), raw.readDeadlines...)
	raw.mu.Unlock()
	if len(deadlines) < 2 {
		t.Fatalf("read deadline calls = %v, want idle probe plus restoration", deadlines)
	}
	if !deadlines[len(deadlines)-1].IsZero() {
		t.Fatalf("final read deadline = %v, want cleared deadline", deadlines[len(deadlines)-1])
	}

	empty, err := NewConnByNetConn(&issue83ScriptedConn{reads: []issue83ReadStep{{err: io.EOF}}}).Recv(-1, nil)
	if err != nil || len(empty) != 0 {
		t.Fatalf("Recv(-1) empty EOF = (%q, %v), want empty success", empty, err)
	}
}
