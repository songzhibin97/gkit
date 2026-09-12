package watching

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// gcHeapEventIDs returns the event ID embedded in every gcHeap binary dump
// found in dir. A collision suffix may follow the timestamp, but the event ID
// stays in the second component. Every file must be an independently valid pprof.
func gcHeapEventIDs(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, type2name[gcHeap]+".") || !strings.HasSuffix(name, ".bin") {
			continue
		}
		parts := strings.Split(name, ".")
		if len(parts) < 2 {
			t.Fatalf("unexpected gcHeap dump file name: %q", name)
		}
		ids = append(ids, parts[1])
		assertReadableProfile(t, filepath.Join(dir, name))
	}
	return ids
}

func TestGCHeapDumpEventIDUsesGCHeapTriggerCount(t *testing.T) {
	dir := t.TempDir()
	w := NewWatching(WithDumpPath(dir, "watching.log"), WithBinaryDump())
	t.Cleanup(func() { w.setLogger(os.Stdout) })
	w.gcHeapStats = newRing(1)
	w.gcHeapStats.push(1)

	// Recognisably different counters: the event ID must follow the gcHeap one.
	w.grTriggerCount = 7
	w.gcHeapTriggerCount = 3

	if !w.gcHeapProfile(1, true, typeConfig{Enable: true}) {
		t.Fatal("forced gcHeapProfile did not dump")
	}

	ids := gcHeapEventIDs(t, dir)
	if len(ids) != 1 {
		t.Fatalf("gcHeap dump files = %v, want exactly one", ids)
	}
	if got, want := ids[0], "heap-3"; got != want {
		t.Fatalf("gcHeap dump event ID = %q, want %q (goroutine counter is %d)", got, want, w.grTriggerCount)
	}
}

func TestGCHeapPairedDumpsShareOneEventID(t *testing.T) {
	dir := t.TempDir()
	w := NewWatching(WithDumpPath(dir, "watching.log"), WithBinaryDump())
	t.Cleanup(func() { w.setLogger(os.Stdout) })
	w.gcHeapStats = newRing(1)
	w.gcHeapStats.push(1)

	w.grTriggerCount = 7
	w.gcHeapTriggerCount = 3

	// gcHeapProfile dumps twice per trigger; gcHeapCheckAndDump only bumps
	// gcHeapTriggerCount after the second one. A goroutine dump firing between
	// the pair bumps grTriggerCount, which must not affect the heap event ID.
	if !w.gcHeapProfile(1, true, typeConfig{Enable: true}) {
		t.Fatal("first forced gcHeapProfile did not dump")
	}
	w.grTriggerCount = 9
	if !w.gcHeapProfile(1, true, typeConfig{Enable: true}) {
		t.Fatal("second forced gcHeapProfile did not dump")
	}

	ids := gcHeapEventIDs(t, dir)
	if len(ids) != 2 {
		t.Fatalf("gcHeap dump files = %v, want exactly two", ids)
	}
	for _, id := range ids {
		if id != "heap-3" {
			t.Fatalf("paired gcHeap dumps have event IDs %v, want all %q", ids, "heap-3")
		}
	}
}
