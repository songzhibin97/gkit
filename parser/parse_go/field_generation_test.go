package parse_go

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const fieldFixture = `package fixture

type Child struct {
	Label string
}

type Worker interface {
	Work()
}

type Alias string

type Parent struct {
	A, B int
	Nested Child
	Worker Worker
	Inline interface{}
	Flags map[bool]uint16
	Tiny int8
	Count uint
	Small uint8
	Medium uint16
	Alias Alias
}
`

func TestParseStructIncludesAllNamedFields(t *testing.T) {
	_, fields := parseFieldFixture(t)
	assertFieldType(t, fields, "A", "int64")
	assertFieldType(t, fields, "B", "int64")
}

func TestParseStructKeepsLocalMessagesAndSkipsUnsupportedTypes(t *testing.T) {
	_, fields := parseFieldFixture(t)
	assertFieldType(t, fields, "Nested", "Child")
	for _, name := range []string{"Worker", "Inline", "Alias"} {
		if _, ok := fields[name]; ok {
			t.Fatalf("unsupported field %q was generated as %q", name, fields[name])
		}
	}
}

func TestParseStructSupportsBoolMapKeys(t *testing.T) {
	_, fields := parseFieldFixture(t)
	assertFieldType(t, fields, "Flags", "map<bool,uint32>")
}

func TestParseStructMapsNarrowIntegerScalars(t *testing.T) {
	parsed, fields := parseFieldFixture(t)
	want := map[string]string{
		"Tiny":   "int32",
		"Count":  "uint32",
		"Small":  "uint32",
		"Medium": "uint32",
	}
	for name, typ := range want {
		assertFieldType(t, fields, name, typ)
	}

	generated := parsed.Generate()
	for name, typ := range want {
		line := typ + " " + name + " = "
		if !strings.Contains(generated, line) {
			t.Fatalf("generated proto missing %q:\n%s", line, generated)
		}
	}
	compileGeneratedProto(t, generated)
}

func parseFieldFixture(t *testing.T) (*GoParsePB, map[string]string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "fixture.go")
	if err := os.WriteFile(path, []byte(fieldFixture), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	parserValue, err := ParseGo(path, func(value interface{}) {
		value.(*GoParsePB).Metas["ServerName"] = "FixtureService"
	})
	if err != nil {
		t.Fatalf("ParseGo() error = %v", err)
	}
	parsed := parserValue.(*GoParsePB)
	var parent *Message
	for _, message := range parsed.Messages() {
		if message.Name == "Parent" {
			parent = message
			break
		}
	}
	if parent == nil {
		t.Fatal("Parent message was not parsed")
	}

	fields := make(map[string]string, len(parent.Files))
	for _, field := range parent.Files {
		fields[field.Name] = field.TypePB
	}
	return parsed, fields
}

func assertFieldType(t *testing.T, fields map[string]string, name, want string) {
	t.Helper()
	got, ok := fields[name]
	if !ok {
		t.Fatalf("field %q is missing; parsed fields = %v", name, fields)
	}
	if got != want {
		t.Fatalf("field %q type = %q, want %q", name, got, want)
	}
}

func compileGeneratedProto(t *testing.T, generated string) {
	t.Helper()
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is not installed")
	}
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "generated.proto")
	if err := os.WriteFile(protoPath, []byte(generated), 0o600); err != nil {
		t.Fatalf("write generated proto: %v", err)
	}
	descriptorPath := filepath.Join(dir, "generated.pb")
	cmd := exec.Command(
		protoc,
		"--proto_path="+dir,
		"--descriptor_set_out="+descriptorPath,
		protoPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s\nGenerated proto:\n%s", err, output, generated)
	}
}
