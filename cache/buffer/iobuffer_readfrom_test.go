package buffer

import (
	"errors"
	"io"
	"testing"
)

type readFromStep struct {
	data string
	err  error
}

// readFromReader deliberately implements only io.Reader, so io.Copy dispatches
// to the destination's ReadFrom instead of a source WriteTo method.
type readFromReader struct {
	steps []readFromStep
}

func (r *readFromReader) Read(p []byte) (int, error) {
	if len(r.steps) == 0 {
		return 0, io.EOF
	}
	step := &r.steps[0]
	n := copy(p, step.data)
	step.data = step.data[n:]
	if len(step.data) > 0 {
		return n, nil
	}
	err := step.err
	r.steps = r.steps[1:]
	return n, err
}

// Regression for #164, item 01-01: a temporary (0, nil) read is not EOF.
func TestIoBufferReadFromTemporaryEmptyReads(t *testing.T) {
	readErr := errors.New("source read failed")
	for _, test := range []struct {
		name    string
		steps   []readFromStep
		want    string
		wantErr error
	}{
		{
			name:  "empty_reads_before_data",
			steps: []readFromStep{{}, {}, {data: "payload"}, {err: io.EOF}},
			want:  "payload",
		},
		{
			name:  "empty_read_between_data_with_eof",
			steps: []readFromStep{{data: "first"}, {}, {data: "last", err: io.EOF}},
			want:  "firstlast",
		},
		{
			name:  "empty_read_then_eof",
			steps: []readFromStep{{}, {err: io.EOF}},
		},
		{
			name:    "empty_read_then_error",
			steps:   []readFromStep{{}, {err: readErr}},
			wantErr: readErr,
		},
		{
			name:    "empty_read_between_data_with_error",
			steps:   []readFromStep{{data: "first"}, {}, {data: "last", err: readErr}},
			want:    "firstlast",
			wantErr: readErr,
		},
		{
			name:    "empty_read_after_data_then_error",
			steps:   []readFromStep{{data: "payload"}, {}, {err: readErr}},
			want:    "payload",
			wantErr: readErr,
		},
	} {
		for _, useCopy := range []bool{false, true} {
			method := "ReaderFrom"
			if useCopy {
				method = "Copy"
			}
			t.Run(method+"/"+test.name, func(t *testing.T) {
				buf := NewIoBuffer(16)
				defer func() {
					if err := PutIoPool(buf); err != nil {
						t.Error(err)
					}
				}()
				const prefix = "existing:"
				if _, err := buf.WriteString(prefix); err != nil {
					t.Fatal(err)
				}
				source := &readFromReader{steps: append([]readFromStep(nil), test.steps...)}
				var n int64
				var err error
				if useCopy {
					n, err = io.Copy(buf, source)
				} else {
					var destination io.ReaderFrom = buf
					n, err = destination.ReadFrom(source)
				}
				if n != int64(len(test.want)) || err != test.wantErr {
					t.Errorf("read = (%d, %v), want (%d, %v)", n, err, len(test.want), test.wantErr)
				}
				if got := string(buf.Bytes()); got != prefix+test.want {
					t.Errorf("buffer = %q, want %q", got, prefix+test.want)
				}
				if len(source.steps) != 0 {
					t.Errorf("returned before terminal error: %d unread steps", len(source.steps))
				}
			})
		}
	}
}
