package cpu

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestReadLinesEOFAndLimits(t *testing.T) {
	for _, tt := range []struct {
		name, data string
		offset     uint
		n          int
		want       []string
	}{
		{"unterminated", "first\nlast", 0, -1, []string{"first", "last"}},
		{"terminated", "first\nlast\n", 0, -1, []string{"first", "last"}},
		{"single", "last", 0, -1, []string{"last"}},
		{"empty", "", 0, -1, nil},
		{"blank", "\n\nlast", 0, -1, []string{"", "", "last"}},
		{"offset_last", "first\nlast", 1, 1, []string{"last"}},
		{"offset_all", "first\nlast", 1, -1, []string{"last"}},
		{"bounded", "first\nlast", 0, 1, []string{"first"}},
		{"zero", "first\nlast", 0, 0, nil},
		{"offset_zero", "first\nlast", 1, 0, nil},
		{"past_end", "first\nlast", 3, -1, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "lines")
			if err := os.WriteFile(path, []byte(tt.data), 0600); err != nil {
				t.Fatal(err)
			}
			got, err := readLinesOffsetN(path, tt.offset, tt.n)
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %#v, %v; want %#v, nil", got, err, tt.want)
			}
			if tt.offset == 0 && tt.n == -1 {
				got, err = readLines(path)
				if err != nil || !reflect.DeepEqual(got, tt.want) {
					t.Fatalf("readLines = %#v, %v; want %#v, nil", got, err, tt.want)
				}
			}
		})
	}
}

func TestReadLinesReadError(t *testing.T) {
	// Opening a directory succeeds, but reading file contents fails.
	path := t.TempDir()
	_, err := readLines(path)
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) || pathErr.Op != "read" || pathErr.Path != path {
		t.Fatalf("read error = %v, want underlying read PathError for %q", err, path)
	}
}
