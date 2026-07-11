package parse_go

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoRejectsTaggedRPCWithoutRequest(t *testing.T) {
	path := writeGoFixture(t, `package fixture

type Response struct{}

// @service:Fixture
// @method:get
// @router:/missing-request
func MissingRequest() Response { return Response{} }
`)

	_, err := ParseGo(path, AddParseFunc(parseDoc), AddParseStruct(parseTag))
	assertRPCSignatureError(t, err, "MissingRequest", "request")
}

func TestParseGoRejectsTaggedRPCWithoutResponse(t *testing.T) {
	path := writeGoFixture(t, `package fixture

type Request struct{}

// @service:Fixture
// @method:post
// @router:/missing-response
func MissingResponse(req Request) {}
`)

	_, err := ParseGo(path, AddParseFunc(parseDoc), AddParseStruct(parseTag))
	assertRPCSignatureError(t, err, "MissingResponse", "response")
}

func TestParseGoAcceptsContextPointerRPCAndSkipsHelper(t *testing.T) {
	path := writeGoFixture(t, `package fixture

import "context"

type Request struct{}
type Response struct{}

func helper() {}

// @service:Fixture
// @method:post
// @router:/register
func Register(ctx context.Context, req *Request) (*Response, error) { return nil, nil }
`)

	parsed, err := ParseGo(path, AddParseFunc(parseDoc), AddParseStruct(parseTag))
	if err != nil {
		t.Fatalf("ParseGo() error = %v", err)
	}
	generated := parsed.(*GoParsePB).Generate()
	if !strings.Contains(generated, "rpc Register (Request) returns (Response)") {
		t.Fatalf("generated proto is missing the valid RPC signature:\n%s", generated)
	}
	if strings.Contains(generated, "rpc helper") {
		t.Fatalf("generated proto contains ordinary helper as RPC:\n%s", generated)
	}
}

func TestParseGoProseMentionDoesNotCreateRPC(t *testing.T) {
	path := writeGoFixture(t, `package fixture

// This ordinary helper only mentions @method:post in prose.
func helper() {}
`)

	parsed, err := ParseGo(path, AddParseFunc(parseDoc))
	if err != nil {
		t.Fatalf("ParseGo() error = %v", err)
	}
	generated := parsed.(*GoParsePB).Generate()
	if strings.Contains(generated, "rpc helper") {
		t.Fatalf("generated proto contains prose-only helper as RPC:\n%s", generated)
	}
}

func TestParseGoCustomParserRPCIsGenerated(t *testing.T) {
	path := writeGoFixture(t, `package fixture

type Request struct{}
type Response struct{}

// custom transport metadata
func Custom(req *Request) (*Response, error) { return nil, nil }
`)
	customParser := func(server *Server) {
		if server.Name == "Custom" {
			server.ServerName = "Fixture"
			server.Method = "post"
			server.Router = "/custom"
		}
	}

	parsed, err := ParseGo(path, AddParseFunc(customParser), AddParseStruct(parseTag))
	if err != nil {
		t.Fatalf("ParseGo() error = %v", err)
	}
	generated := parsed.(*GoParsePB).Generate()
	if !strings.Contains(generated, "rpc Custom (Request) returns (Response)") {
		t.Fatalf("generated proto is missing custom-parser RPC:\n%s", generated)
	}
}

func TestParseGoRejectsCustomParserRPCInvalidSignatures(t *testing.T) {
	tests := []struct {
		name         string
		functionName string
		detail       string
		source       string
	}{
		{
			name:         "missing request",
			functionName: "CustomMissingRequest",
			detail:       "request",
			source: `package fixture

type Response struct{}

// custom transport metadata
func CustomMissingRequest() Response { return Response{} }
`,
		},
		{
			name:         "missing response",
			functionName: "CustomMissingResponse",
			detail:       "response",
			source: `package fixture

type Request struct{}

// custom transport metadata
func CustomMissingResponse(req Request) {}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := writeGoFixture(t, tt.source)
			customParser := func(server *Server) {
				server.ServerName = "Fixture"
				server.Method = "post"
				server.Router = "/custom"
			}

			_, err := ParseGo(path, AddParseFunc(customParser), AddParseStruct(parseTag))
			assertRPCSignatureError(t, err, tt.functionName, tt.detail)
		})
	}
}

func TestParseGoDemoRPCSignaturesRemainValid(t *testing.T) {
	parsed, err := ParseGo("../demo/demo.api", AddParseFunc(parseDoc), AddParseStruct(parseTag))
	if err != nil {
		t.Fatalf("ParseGo(demo.api) error = %v", err)
	}
	generated := parsed.(*GoParsePB).Generate()
	for _, signature := range []string{
		"rpc Register (Request) returns (Response)",
		"rpc Register2 (Request) returns (Response)",
	} {
		if !strings.Contains(generated, signature) {
			t.Fatalf("generated demo proto is missing %q:\n%s", signature, generated)
		}
	}
}

func writeGoFixture(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.go")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return path
}

func assertRPCSignatureError(t *testing.T, err error, functionName, detail string) {
	t.Helper()
	if err == nil {
		t.Fatal("ParseGo() error = nil, want unsupported RPC signature error")
	}
	for _, want := range []string{"unsupported RPC signature", functionName, detail} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("ParseGo() error = %q, want it to contain %q", err, want)
		}
	}
}
