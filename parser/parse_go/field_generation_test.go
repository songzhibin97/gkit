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
	Octet byte
	CodePoint rune
	UintList []uint
	ByteList []byte
	RuneList []rune
	UintValues map[string]uint
	ByteValues map[string]byte
	RuneValues map[string]rune
	UintKeys map[uint]string
	ByteKeys map[byte]string
	RuneKeys map[rune]string
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

func TestParseStructMapsIntegerScalars(t *testing.T) {
	parsed, fields := parseFieldFixture(t)
	want := []struct {
		name string
		typ  string
	}{
		{name: "Tiny", typ: "int32"},
		{name: "Count", typ: "uint64"},
		{name: "Small", typ: "uint32"},
		{name: "Medium", typ: "uint32"},
		{name: "Octet", typ: "uint32"},
		{name: "CodePoint", typ: "int32"},
	}
	for _, test := range want {
		test := test
		t.Run(test.name, func(t *testing.T) {
			assertFieldType(t, fields, test.name, test.typ)
		})
	}

	generated := parsed.Generate()
	for _, test := range want {
		line := test.typ + " " + test.name + " = "
		if !strings.Contains(generated, line) {
			t.Fatalf("generated proto missing %q:\n%s", line, generated)
		}
	}
	compileGeneratedProto(t, generated)
}

func TestParseStructMapsIntegerSlicesAndMapValues(t *testing.T) {
	_, fields := parseFieldFixture(t)
	want := map[string]string{
		"UintList":   "repeated uint64",
		"ByteList":   "bytes",
		"RuneList":   "repeated int32",
		"UintValues": "map<string,uint64>",
		"ByteValues": "map<string,uint32>",
		"RuneValues": "map<string,int32>",
	}
	for name, typ := range want {
		assertFieldType(t, fields, name, typ)
	}
}

func TestParseStructMapsLegalIntegerMapKeys(t *testing.T) {
	_, fields := parseFieldFixture(t)
	want := map[string]string{
		"UintKeys": "map<uint64,string>",
		"ByteKeys": "map<uint32,string>",
		"RuneKeys": "map<int32,string>",
	}
	for name, typ := range want {
		assertFieldType(t, fields, name, typ)
	}
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
