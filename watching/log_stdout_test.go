package watching_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/watching"
)

// Use a subprocess so replacing os.Stdout cannot affect other tests. The
// redirected file is owned by the test; this never modifies /dev/stdout.
func TestStartPreservesRedirectedStdout(t *testing.T) {
	if path := os.Getenv("GKIT_STDOUT_REGRESSION_FILE"); path != "" {
		f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
		if err != nil {
			t.Fatal(err)
		}
		// Keep this process-owned handle installed until the subprocess exits:
		// Stop does not join background diagnostics that can still use stdout.
		os.Stdout = f
		w := watching.NewWatching(watching.WithLoggerSplit(true, "1KB"), watching.WithCollectInterval("1h"))
		w.Start()
		w.Start() // The public repeated-start path must log to the same sink.
		w.Stop()
		return
	}

	for _, tc := range []struct {
		name string
		size int
	}{
		{"above_threshold", 2048},
		{"below_threshold", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "stdout.log")
			seed := bytes.Repeat([]byte("x"), tc.size)
			if err := os.WriteFile(path, seed, 0600); err != nil {
				t.Fatal(err)
			}
			// Keep the original sink open, so reading it still detects log
			// diversion even if rotation renames the path behind our back.
			sink, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer sink.Close()
			before, err := sink.Stat()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestStartPreservesRedirectedStdout$")
			cmd.Env = append(os.Environ(), "GKIT_STDOUT_REGRESSION_FILE="+path)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("public Start subprocess: %v\n%s", err, out)
			}
			after, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) {
				t.Error("Start replaced the redirected stdout file")
			}
			data, err := io.ReadAll(sink)
			if err != nil {
				t.Fatal(err)
			}
			text := string(data)
			if !strings.HasPrefix(text, string(seed)) {
				t.Error("Start changed existing stdout content")
			}
			for _, message := range []string{
				"[Watching] use the default memory percent calculated by gopsutil",
				"Watching has started, please don't start it again.",
			} {
				if !strings.Contains(text, message) {
					t.Errorf("original stdout is missing %q", message)
				}
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != "stdout.log" {
				t.Errorf("Start created rotation artifacts: %v", entries)
			}
		})
	}
}
