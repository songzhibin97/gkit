package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestGeneratedAssembly(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal", "avo")
	if err := os.MkdirAll(dir, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sse.go", "avx.go", "gen.go", "build.sh", "go.mod", "go.sum"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	cmd := exec.Command("sh", "build.sh")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build.sh: %v\n%s", err, out)
	}
	for _, name := range []string{"avx2_amd64.s", "sse2_amd64.s"} {
		got, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("..", "..", name))
		if err != nil {
			t.Fatal(err)
		}
		// Compare every non-comment line, including symbols, data, operands,
		// register allocation and instruction order. Headers are not byte equal.
		body := func(data []byte) string {
			var lines []string
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if line != "" && !strings.HasPrefix(line, "//") {
					lines = append(lines, line)
				}
			}
			return strings.Join(lines, "\n")
		}
		if body(got) != body(want) {
			t.Errorf("%s generated assembly differs from checked-in body", name)
		}
	}
}
