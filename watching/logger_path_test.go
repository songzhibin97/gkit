package watching

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRotationUsesOpenedLoggerPath(t *testing.T) {
	for _, tc := range []struct {
		name      string
		logName   string
		unrelated bool
	}{
		{"default", defaultLoggerName, false},
		{"custom", "custom.log", false},
		{"custom with default file", "custom.log", true},
		{"nested custom", "nested/custom.log", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			w := NewWatching(WithDumpPath(dir, tc.logName), WithLoggerSplit(true, "1B"))
			t.Cleanup(func() { w.setLogger(os.Stdout) })
			src := filepath.Join(dir, tc.logName)
			unrelated := filepath.Join(filepath.Dir(src), defaultLoggerName)
			if tc.unrelated {
				if err := os.WriteFile(unrelated, []byte("unrelated\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			const batch = "completed batch\n"
			w.writeString(batch)
			if !w.rotateEnabled() {
				t.Fatal("rotation disabled for a valid logger path")
			}
			ref := w.acquireLogger()
			name := ref.file.Name()
			ref.release()
			if name != src {
				t.Fatalf("active logger = %q, want %q", name, src)
			}
			current, err := os.ReadFile(src)
			if err != nil {
				t.Fatal(err)
			}
			if len(current) != 0 {
				t.Fatalf("active log not rotated: %q", current)
			}
			backups, err := filepath.Glob(src + "_*.back")
			if err != nil {
				t.Fatal(err)
			}
			if len(backups) != 1 {
				t.Fatalf("backups = %v, want one", backups)
			}
			content, err := os.ReadFile(backups[0])
			if err != nil {
				t.Fatal(err)
			}
			if string(content) != batch {
				t.Fatalf("backup = %q, want %q", content, batch)
			}
			if tc.unrelated {
				content, err := os.ReadFile(unrelated)
				if err != nil {
					t.Fatal(err)
				}
				if string(content) != "unrelated\n" {
					t.Fatalf("unrelated default log changed: %q", content)
				}
			}
		})
	}
}
