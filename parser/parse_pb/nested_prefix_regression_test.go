package parse_pb

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNestedMessageAndEnumPrefixesFollowOuterToInnerOrder(t *testing.T) {
	protoFile := filepath.Join(t.TempDir(), "nested.proto")
	definition := `syntax = "proto3";
package nested;
message A {
  message B {
    message C { string value = 1; }
    enum D { D_UNSPECIFIED = 0; }
  }
}`
	if err := os.WriteFile(protoFile, []byte(definition), 0o600); err != nil {
		t.Fatal(err)
	}

	parsed, err := ParsePb(protoFile)
	if err != nil {
		t.Fatal(err)
	}
	model := parsed.(*PbParseGo)
	for _, name := range []string{"A", "AB", "ABC"} {
		if _, ok := model.Message[name]; !ok {
			t.Fatalf("message %q missing; got %#v", name, mapKeys(model.Message))
		}
	}
	if _, ok := model.Enums["ABD"]; !ok {
		t.Fatalf("enum %q missing; got %#v", "ABD", enumMapKeys(model.Enums))
	}
	for _, wrong := range []string{"BAC", "BAD"} {
		if _, ok := model.Message[wrong]; ok {
			t.Fatalf("reversed-prefix message %q was generated", wrong)
		}
		if _, ok := model.Enums[wrong]; ok {
			t.Fatalf("reversed-prefix enum %q was generated", wrong)
		}
	}
}

func mapKeys(values map[string]*Message) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func enumMapKeys(values map[string]*Enum) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}
