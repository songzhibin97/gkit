package parse_pb

import (
	"context"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestParsePb(t *testing.T) {
	path := validatedProtoFixture(t, `syntax = "proto3";
package fixture;
message Item { string name = 1; }
`)
	r, err := ParsePb(path)
	if err != nil {
		t.Fatal(err)
	}
	compileGeneratedGo(t, r.Generate(), "package fixture\nvar _ = Item{Name: \"kept\"}\n")
}

func validatedProtoFixture(t *testing.T, definition string) string {
	t.Helper()
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is not installed")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.proto")
	if err := os.WriteFile(path, []byte(definition), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, protoc, "--proto_path="+dir, "--descriptor_set_out="+filepath.Join(dir, "fixture.pb"), path)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc rejected fixture: %v\n%s\n%s", err, output, definition)
	}
	return path
}

func compileGeneratedGo(t *testing.T, generated, consumer string) {
	t.Helper()
	dir := t.TempDir()
	fset := token.NewFileSet()
	var files []*ast.File
	var paths []string
	for i, source := range []string{generated, consumer} {
		name := []string{"generated.go", "consumer.go"}[i]
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(fset, path, source, parser.AllErrors)
		if err != nil {
			t.Fatalf("parse generated consumer: %v\n%s", err, generated)
		}
		files = append(files, file)
		paths = append(paths, path)
	}
	config := types.Config{}
	if _, err := config.Check("fixture", fset, files, nil); err != nil {
		t.Fatalf("typecheck generated consumer: %v\n%s", err, generated)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	args := append([]string{"tool", "compile", "-o", filepath.Join(dir, "fixture.o")}, paths...)
	if output, err := exec.CommandContext(ctx, "go", args...).CombinedOutput(); err != nil {
		t.Fatalf("compile generated consumer: %v\n%s\n%s", err, output, generated)
	}
}
