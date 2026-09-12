package watching

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/pprof"
	"sync"
	"testing"
	"time"
)

func assertReadableProfile(t *testing.T, name string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, "go", "tool", "pprof", "-top", name).CombinedOutput(); err != nil {
		t.Fatalf("pprof cannot parse %s: %v\n%s", name, err, output)
	}
}

func TestBinaryFileCollisionPreservesProfiles(t *testing.T) {
	name := filepath.Join(t.TempDir(), "gcHeap.heap-3.20260912120000.000.bin")
	existing := []byte("an earlier dump must remain unchanged")
	if err := os.WriteFile(name, existing, 0o600); err != nil {
		t.Fatal(err)
	}
	var profile bytes.Buffer
	if err := pprof.Lookup("heap").WriteTo(&profile, 0); err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{name: true}
	for i := 0; i < 2; i++ {
		f, created, err := createBinaryFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if seen[created] {
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			t.Fatalf("creation reused occupied profile path %s", created)
		}
		seen[created] = true
		if _, err := f.Write(profile.Bytes()); err != nil {
			if closeErr := f.Close(); closeErr != nil {
				t.Errorf("close failed profile: %v", closeErr)
			}
			t.Fatal(err)
		}
		if err := f.Close(); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(created)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(data, profile.Bytes()) {
			t.Fatalf("profile %s was appended or truncated", created)
		}
		assertReadableProfile(t, created)
	}
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, existing) {
		t.Fatal("the pre-existing dump was modified")
	}
}

func TestBinaryFileConcurrentCollision(t *testing.T) {
	const writers = 32
	name := filepath.Join(t.TempDir(), "cpu..20260912120000.000.bin")
	start := make(chan struct{})
	var wg sync.WaitGroup
	names := make([]string, writers)
	writeErrors := make([]error, writers)
	for i := range names {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			f, created, err := createBinaryFile(name)
			if err != nil {
				writeErrors[i] = err
				return
			}
			names[i] = created
			_, err = fmt.Fprintf(f, "writer-%d", i)
			writeErrors[i] = errors.Join(err, f.Close())
		}(i)
	}
	close(start)
	wg.Wait()
	seen := make(map[string]bool)
	for i, name := range names {
		if writeErrors[i] != nil {
			t.Fatal(writeErrors[i])
		}
		if seen[name] {
			t.Fatalf("concurrent writers shared %s", name)
		}
		seen[name] = true
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != fmt.Sprintf("writer-%d", i) {
			t.Fatalf("writer %d read back %q", i, data)
		}
	}
}

func TestBinaryFileCollisionRetriesAreBounded(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "gcHeap.heap-3.20260912120000.000")
	for i := 0; i < 1000; i++ {
		name := base + ".bin"
		if i > 0 {
			name = fmt.Sprintf("%s.%d.bin", base, i)
		}
		if err := os.WriteFile(name, []byte("occupied"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	f, _, err := createBinaryFile(base + ".bin")
	if f != nil {
		if closeErr := f.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		t.Fatal("collision limit unexpectedly opened a file")
	}
	if !errors.Is(err, os.ErrExist) {
		t.Fatalf("collision limit error = %v, want os.ErrExist", err)
	}
}

func TestBinaryFileDirectoryHandling(t *testing.T) {
	dir := t.TempDir()
	name := filepath.Join(dir, "nested", "gcHeap.heap-0.20260912120000.000.bin")
	f, created, err := createBinaryFile(name)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if created != name {
		t.Fatalf("non-colliding name changed: %s", created)
	}
	// A regular file cannot be used as a parent directory; do not hide this
	// failure as a filename collision or retry it indefinitely.
	f, _, err = createBinaryFile(filepath.Join(name, "child.bin"))
	if f != nil {
		if closeErr := f.Close(); closeErr != nil {
			t.Fatal(closeErr)
		}
		t.Fatal("created a profile beneath a regular file")
	}
	if err == nil {
		t.Fatal("missing invalid-parent error")
	}
}
