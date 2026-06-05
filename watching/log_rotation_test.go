package watching

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func countBackups(t *testing.T, dir string) int {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	n := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".back") {
			n++
		}
	}
	return n
}

func newRotatingWatching(t *testing.T, dir string, splitSize int64) (*Watching, *os.File) {
	t.Helper()
	f, err := os.OpenFile(filepath.Join(dir, defaultLoggerName), defaultLoggerFlags, defaultLoggerPerm)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	w := &Watching{config: defaultConfig()}
	w.config.DumpPath = dir
	w.config.logConfigs.RotateEnable = true
	w.config.logConfigs.SplitLoggerSize = splitSize
	w.setLogger(f) // retire the os.Stdout default, install the temp file
	return w, f
}

// TestLoggerRef_DeferredCloseUntilLastRelease pins the lifetime contract of
// the rotation fix: retiring the current-slot reference (what setLogger does
// on rotation) must NOT close the file while an in-flight writer still holds
// a reference. The file is closed only when the final reference is released.
// Reverting release() to close eagerly on retire makes the mid-write
// assertion below fail.
func TestLoggerRef_DeferredCloseUntilLastRelease(t *testing.T) {
	dir := t.TempDir()
	f, err := os.OpenFile(filepath.Join(dir, "ref.log"), defaultLoggerFlags, defaultLoggerPerm)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	ref := newLoggerRef(f) // refs=1: the current slot
	ref.acquire()          // refs=2: an in-flight writer snapshots it

	ref.release() // refs=1: rotation retires the current slot — must NOT close
	if _, err := f.WriteString("still-open\n"); err != nil {
		t.Fatalf("file was closed while a writer still held a reference: %v", err)
	}

	ref.release() // refs=0: the writer finishes — now it closes
	if _, err := f.WriteString("after-close\n"); err == nil {
		t.Fatal("file should be closed after the last reference is released")
	}
}

// TestLoggerRef_StdoutNeverClosed guards that os.Stdout is never closed even
// when its reference count reaches zero (the default logger, before a
// WithDumpPath swap retires it).
func TestLoggerRef_StdoutNeverClosed(t *testing.T) {
	ref := newLoggerRef(os.Stdout) // refs=1
	ref.release()                  // refs=0 — must be a no-op for os.Stdout
	if _, err := os.Stdout.Stat(); err != nil {
		t.Fatalf("os.Stdout was closed by release(): %v", err)
	}
}

// TestRotate_CurrentRefRotates is the positive control: rotate() on the
// still-current handle performs exactly one rotation (one .back file).
func TestRotate_CurrentRefRotates(t *testing.T) {
	dir := t.TempDir()
	w, _ := newRotatingWatching(t, dir, defaultShardLoggerSize)

	ref := w.acquireLogger() // current handle
	defer ref.release()

	if got := countBackups(t, dir); got != 0 {
		t.Fatalf("precondition: %d backups, want 0", got)
	}
	w.rotate(ref)
	if got := countBackups(t, dir); got != 1 {
		t.Fatalf("current ref did not rotate: %d backups, want 1", got)
	}
}

// TestRotate_StaleRefSkips guards the stale-rotation fix: a writer whose
// acquired handle was already retired by a concurrent rotation must NOT
// rotate again — otherwise it rotates the new active file off the retired
// handle's size. Removing the staleness re-check in rotate() makes this fail
// (the stale ref produces an extra .back file).
func TestRotate_StaleRefSkips(t *testing.T) {
	dir := t.TempDir()
	w, _ := newRotatingWatching(t, dir, defaultShardLoggerSize)

	staleRef := w.acquireLogger() // points at the current handle
	defer staleRef.release()

	// Simulate another writer rotating the active handle out from under us.
	f2, err := os.OpenFile(filepath.Join(dir, "rotated.tmp"), defaultLoggerFlags, defaultLoggerPerm)
	if err != nil {
		t.Fatalf("open f2: %v", err)
	}
	w.setLogger(f2) // activeLog is now f2; staleRef is retired-but-open

	before := countBackups(t, dir)
	w.rotate(staleRef) // stale ref must skip
	if got := countBackups(t, dir); got != before {
		t.Fatalf("stale ref triggered a rotation: backups %d -> %d", before, got)
	}
}

// TestWriteString_ConcurrentRotationNoUseAfterClose drives many concurrent
// writers against a tiny split size so rotation fires repeatedly while other
// goroutines are mid-write. With the refcount handle, no writer ever touches
// a closed file and the swap introduces no data race. Run under -race; it
// must complete without panic, keep rotation enabled (an eager close would
// make a stale writer hit a closed file in Stat and disable rotation), and
// leave the active logger usable.
func TestWriteString_ConcurrentRotationNoUseAfterClose(t *testing.T) {
	dir := t.TempDir()
	w, _ := newRotatingWatching(t, dir, 1) // 1-byte split forces a rotation attempt on nearly every write

	const writers, perWriter = 8, 64
	var wg sync.WaitGroup
	wg.Add(writers)
	for i := 0; i < writers; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < perWriter; j++ {
				w.writeString("concurrent-rotation-line\n")
			}
		}()
	}
	wg.Wait()

	// Rotation must not have been disabled by a closed/stale-file Stat error.
	w.config.L.RLock()
	stillEnabled := w.config.logConfigs.RotateEnable
	w.config.L.RUnlock()
	if !stillEnabled {
		t.Fatal("rotation was disabled during the storm — a writer hit a closed/stale file")
	}

	// The active logger must still be usable after the storm.
	ref := w.acquireLogger()
	defer ref.release()
	if _, err := ref.file.WriteString("final\n"); err != nil {
		t.Fatalf("active logger unusable after concurrent rotation: %v", err)
	}
}
