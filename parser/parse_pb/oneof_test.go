package parse_pb

import (
	"strings"
	"testing"
)

func TestParsePbRejectsUnsupportedOneof(t *testing.T) {
	for _, definition := range []string{
		`message A { oneof choice { string text = 1; int32 number = 2; } }`,
		`message Outer { message A { oneof choice { string text = 1; int32 number = 2; } } }`,
	} {
		path := validatedProtoFixture(t, `syntax = "proto3"; package fixture; `+definition)
		parsed, err := ParsePb(path)
		if err == nil {
			t.Fatalf("oneof silently accepted: %s", parsed.Generate())
		}
		for _, detail := range []string{"unsupported oneof", "choice", "A"} {
			if !strings.Contains(err.Error(), detail) {
				t.Fatalf("error=%q missing %q", err, detail)
			}
		}
		if parsed != nil {
			t.Fatal("unsupported oneof returned a partial successful model")
		}
	}
}
