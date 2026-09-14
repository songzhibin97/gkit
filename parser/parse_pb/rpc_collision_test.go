package parse_pb

import (
	"strings"
	"testing"
)

func TestParsePbRejectsDuplicateRPCNames(t *testing.T) {
	path := validatedProtoFixture(t, `syntax = "proto3"; package fixture;
message A { int32 number = 1; }
message B { string text = 1; }
service First { rpc Get(A) returns(A); }
service Second { rpc Get(B) returns(B); }
`)
	calls := 0
	parsed, err := ParsePb(path, AddParseService(func(*Server) { calls++ }))
	if err == nil {
		t.Fatalf("colliding RPC silently accepted: %s", parsed.Generate())
	}
	for _, detail := range []string{"duplicate RPC", "Get", "Second"} {
		if !strings.Contains(err.Error(), detail) {
			t.Fatalf("error=%q missing %q", err, detail)
		}
	}
	if parsed != nil {
		t.Fatal("collision returned a partial successful model")
	}
	if calls != 2 {
		t.Fatalf("service hooks called %d times, want 2", calls)
	}
}

func TestParsePbKeepsNoncollidingServices(t *testing.T) {
	path := validatedProtoFixture(t, `syntax = "proto3"; package fixture;
message A { int32 number = 1; }
message B { string text = 1; }
service First { rpc ReadA(A) returns(A); }
service Second { rpc ReadB(B) returns(B); }
`)
	hooked := make(map[string]*Server)
	parsed, err := ParsePb(path, AddParseService(func(server *Server) { hooked[server.Name] = server }))
	if err != nil {
		t.Fatal(err)
	}
	model := parsed.(*PbParseGo)
	for _, name := range []string{"ReadA", "ReadB"} {
		if model.Server[name] == nil || model.Server[name] != hooked[name] {
			t.Fatalf("RPC map key/hook changed: %s", name)
		}
	}
	compileGeneratedGo(t, parsed.Generate(), `package fixture
var _ func(A) A = ReadA
var _ func(B) B = ReadB
var _ = A{Number:7}
var _ = B{Text:"kept"}
`)
}
