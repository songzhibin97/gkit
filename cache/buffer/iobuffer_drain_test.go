package buffer

import (
	"math"
	"testing"
)

// Regression for #164, item 01-03: adding MaxInt to an existing read offset
// must not overflow and corrupt the buffer's readable region.
func TestIoBufferDrainCountBoundaries(t *testing.T) {
	for _, test := range []struct {
		name   string
		offset int
		want   string
	}{
		{name: "negative", offset: -1, want: "bc"},
		{name: "zero", offset: 0, want: "bc"},
		{name: "partial", offset: 1, want: "c"},
		{name: "remaining", offset: 2, want: ""},
		{name: "above_remaining", offset: 3, want: "bc"},
		{name: "max_int", offset: math.MaxInt, want: "bc"},
	} {
		t.Run(test.name, func(t *testing.T) {
			buf := NewIoBufferString("abc")
			defer buf.Free()
			buf.Drain(1)
			buf.Drain(test.offset)
			if got := buf.Len(); got != len(test.want) {
				t.Fatalf("Len after Drain(%d) = %d, want %d", test.offset, got, len(test.want))
			}
			if got := string(buf.Bytes()); got != test.want {
				t.Fatalf("Bytes after Drain(%d) = %q, want %q", test.offset, got, test.want)
			}
			if err := buf.Append([]byte("d")); err != nil {
				t.Fatal(err)
			}
			if got := string(buf.Bytes()); got != test.want+"d" {
				t.Fatalf("Bytes after Append = %q, want %q", got, test.want+"d")
			}
			buf.Drain(len(test.want) + 1)
			if buf.Len() != 0 || len(buf.Bytes()) != 0 {
				t.Fatalf("valid final Drain left Len=%d, Bytes=%q", buf.Len(), buf.Bytes())
			}
		})
	}
}

func TestIoBufferInvalidDrainPreservesMark(t *testing.T) {
	for _, test := range []struct {
		name   string
		offset int
	}{
		{name: "negative", offset: -1},
		{name: "above_remaining", offset: 2},
		{name: "max_int", offset: math.MaxInt},
	} {
		t.Run(test.name, func(t *testing.T) {
			buf := NewIoBufferString("abc").(*ioBuffer)
			defer buf.Free()
			buf.Drain(1)
			buf.Mark()
			var read [1]byte
			if n, err := buf.Read(read[:]); n != 1 || err != nil || read[0] != 'b' {
				t.Fatalf("Read after Mark = (%d, %v, %q), want (1, nil, b)", n, err, read)
			}
			buf.Drain(test.offset)
			if got := buf.Len(); got != 1 {
				t.Fatalf("invalid Drain changed Len to %d, want 1", got)
			}
			if got := string(buf.Bytes()); got != "c" {
				t.Fatalf("invalid Drain changed Bytes to %q, want c", got)
			}
			buf.Restore()
			if buf.Len() != 2 || string(buf.Bytes()) != "bc" {
				t.Fatalf("Restore after invalid Drain = (Len %d, Bytes %q), want (2, bc)", buf.Len(), buf.Bytes())
			}
			buf.Drain(1)
			if buf.Len() != 1 || string(buf.Bytes()) != "c" {
				t.Fatalf("valid Drain after Restore = (Len %d, Bytes %q), want (1, c)", buf.Len(), buf.Bytes())
			}
		})
	}
}
