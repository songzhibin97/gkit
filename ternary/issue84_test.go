package ternary

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestTypeGeneratorProducesBuildableSource(t *testing.T) {
	dir := t.TempDir()
	copyFile := func(name string) {
		t.Helper()
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			t.Fatalf("os.WriteFile(%q) error = %v", name, err)
		}
	}
	copyFile("type_gen.go")
	copyFile("template.txt")
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module example.com/generated-ternary\n\ngo 1.20\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(go.mod) error = %v", err)
	}

	generate := exec.Command("go", "run", "type_gen.go")
	generate.Dir = dir
	if output, err := generate.CombinedOutput(); err != nil {
		t.Fatalf("go run type_gen.go error = %v\n%s", err, output)
	}

	build := exec.Command("go", "test", "./...")
	build.Dir = dir
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("generated source does not build: %v\n%s", err, output)
	}
}
