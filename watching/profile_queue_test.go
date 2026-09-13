package watching

import (
	"bytes"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

type unusedProfileReporter struct{}

func (*unusedProfileReporter) Report(string, []byte, string, string) error { return nil }

func TestQueuedThreadProfilesRetainCapturedBytes(t *testing.T) {
	dir := t.TempDir()
	w := NewWatching(WithDumpPath(dir), WithBinaryDump(), WithProfileReporter(&unusedProfileReporter{}))
	t.Cleanup(func() { w.setLogger(os.Stdout) })
	// Model a running monitor while its reporter worker has not yet been
	// scheduled. This legal queue delay needs no background monitor or upload.
	atomic.StoreInt64(&w.stopped, 0)
	w.rptEventsCh = make(chan rptEvent, 2)
	w.threadStats = newRing(1)
	w.threadStats.push(1)
	w.threadTriggerCount = 7
	if !w.threadProfile(2, typeConfig{TriggerMin: 1, TriggerAbs: 1}) {
		t.Fatal("thread profile did not trigger")
	}
	if len(w.rptEventsCh) != 2 {
		t.Fatalf("queued events = %d, want 2", len(w.rptEventsCh))
	}
	var captures [][]byte
	for _, kind := range []string{"thread", "goroutine"} {
		event := <-w.rptEventsCh
		if event.PType != kind || event.EventID != "thr-7" || event.Reason != "curVal [2] > ruleAbs [1]" {
			t.Fatalf("unexpected queued metadata: type=%q id=%q reason=%q", event.PType, event.EventID, event.Reason)
		}
		files, err := filepath.Glob(filepath.Join(dir, kind+".thr-7.*.bin"))
		if err != nil {
			t.Fatal(err)
		}
		if len(files) != 1 {
			t.Fatalf("%s capture files = %v, want exactly one", kind, files)
		}
		captured, err := os.ReadFile(files[0])
		if err != nil {
			t.Fatal(err)
		}
		if len(captured) == 0 {
			t.Fatalf("%s capture is empty", kind)
		}
		captures = append(captures, captured)
		if !bytes.Equal(event.Buf, captured) {
			t.Errorf("queued %s profile differs from same capture: queued=%d, file=%d bytes", kind, len(event.Buf), len(captured))
		}
		assertReadableProfile(t, files[0])
		queuedPath := filepath.Join(dir, kind+"-queued.bin")
		if err := os.WriteFile(queuedPath, event.Buf, 0o600); err != nil {
			t.Fatal(err)
		}
		assertReadableProfile(t, queuedPath)
	}
	if bytes.Equal(captures[0], captures[1]) {
		t.Fatal("thread and goroutine controls unexpectedly identical")
	}
}
