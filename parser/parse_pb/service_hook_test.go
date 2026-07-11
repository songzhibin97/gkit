package parse_pb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseServiceHookRunsWithAndWithoutServiceComments(t *testing.T) {
	protoFile := filepath.Join(t.TempDir(), "services.proto")
	definition := `syntax = "proto3";
package hooktest;
message Request {}
message Response {}

// commented service
service Commented {
  rpc CommentedCall (Request) returns (Response) {}
}

service Plain {
  rpc PlainCall (Request) returns (Response) {}
}`
	if err := os.WriteFile(protoFile, []byte(definition), 0o600); err != nil {
		t.Fatal(err)
	}

	calls := make(map[string]int)
	hooked := make(map[string]*Server)
	parsed, err := ParsePb(protoFile, AddParseService(func(server *Server) {
		calls[server.Name]++
		hooked[server.Name] = server
	}))
	if err != nil {
		t.Fatal(err)
	}
	model := parsed.(*PbParseGo)
	for _, name := range []string{"CommentedCall", "PlainCall"} {
		if calls[name] != 1 {
			t.Fatalf("hook calls for %s = %d, want 1; all calls = %#v", name, calls[name], calls)
		}
		if hooked[name] == nil {
			t.Fatalf("hook did not receive %s", name)
		}
		if model.Server[name] != hooked[name] {
			t.Fatalf("hook object for %s differs from stored server", name)
		}
		if hooked[name].InputParameter != "Request" || hooked[name].OutputParameter != "Response" {
			t.Fatalf("hook object for %s = %#v", name, hooked[name])
		}
	}
	if len(calls) != 2 {
		t.Fatalf("hook received %d distinct servers, want 2: %#v", len(calls), calls)
	}
	commented := hooked["CommentedCall"]
	if len(commented.Notes) != 1 || !strings.Contains(strings.Join(commented.Doc, "\n"), "commented service") {
		t.Fatalf("commented service notes/doc = %#v / %#v", commented.Notes, commented.Doc)
	}
	plain := hooked["PlainCall"]
	if len(plain.Notes) != 0 || len(plain.Doc) != 0 {
		t.Fatalf("plain service unexpectedly inherited notes/doc = %#v / %#v", plain.Notes, plain.Doc)
	}
}
