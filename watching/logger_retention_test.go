package watching

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRepeatedRotationsRetainCompletedBatches(t *testing.T) {
	dir := t.TempDir()
	w := NewWatching(WithDumpPath(dir), WithLoggerSplit(true, "1B"))
	t.Cleanup(func() { w.setLogger(os.Stdout) })
	const batches = 32
	var previousEnd int64 = -1
	sameSecond := false
	for i := 0; i < batches; i++ {
		start := time.Now().Unix()
		w.writeString(fmt.Sprintf("batch-%02d\n", i))
		end := time.Now().Unix()
		if start == end && start == previousEnd {
			sameSecond = true
		}
		previousEnd = -1
		if start == end {
			previousEnd = end
		}
	}
	// This is a bounded clock witness, not a sleep or a skip that can hide the
	// old bug. If the host cannot run two rotations in a second, the case fails.
	if !sameSecond {
		t.Fatal("same-second rotation precondition unverified after 32 normal writes")
	}
	if !w.rotateEnabled() {
		t.Fatal("rotation was disabled")
	}
	backups, err := filepath.Glob(filepath.Join(dir, defaultLoggerName) + "_*.back")
	if err != nil {
		t.Fatal(err)
	}
	contents := make(map[string]int)
	for _, name := range backups {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		contents[string(data)]++
	}
	for i := 0; i < batches; i++ {
		batch := fmt.Sprintf("batch-%02d\n", i)
		if contents[batch] != 1 {
			t.Errorf("completed %q retained %d times, want once", batch, contents[batch])
		}
	}
	if len(backups) != batches {
		t.Errorf("backup count = %d, want %d", len(backups), batches)
	}
	active, err := os.ReadFile(filepath.Join(dir, defaultLoggerName))
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 0 {
		t.Fatalf("last rotation left active bytes: %q", active)
	}
}

func TestRotationPreservesExistingBackups(t *testing.T) {
	dir := t.TempDir()
	w := NewWatching(WithDumpPath(dir), WithLoggerSplit(true, "1B"))
	t.Cleanup(func() { w.setLogger(os.Stdout) })
	// Occupy every legacy timestamp name for a bounded window, including the
	// current second. No timing-dependent retry is used to make the test pass.
	now := time.Now()
	existing := make(map[string]string)
	for i := 0; i < 10; i++ {
		name := filepath.Join(dir, defaultLoggerName) + "_" + now.Add(time.Duration(i)*time.Second).Format("20060102150405") + ".back"
		data := fmt.Sprintf("earlier backup %d\n", i)
		if err := os.WriteFile(name, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		existing[name] = data
	}
	w.writeString("new batch\n")
	if time.Since(now) >= 9*time.Second {
		t.Fatal("occupied timestamp window exceeded; collision witness unverified")
	}
	for name, want := range existing {
		got, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("pre-existing backup %s changed to %q", name, got)
		}
	}
	if !w.rotateEnabled() {
		t.Fatal("rotation disabled on occupied backup name")
	}
	if got := countBackups(t, dir); got != len(existing)+1 {
		t.Errorf("backups = %d, want %d", got, len(existing)+1)
	}
}
