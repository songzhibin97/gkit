package parse_go

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGoRejectsRPCParametersWithoutMessageDeclarations(t *testing.T) {
	tests := []struct {
		name         string
		functionName string
		detail       string
		source       string
	}{
		{
			name:         "string request",
			functionName: "StringRequest",
			detail:       "request",
			source: `package fixture

type Response struct{}

// @service:Fixture
func StringRequest(req string) Response { return Response{} }
`,
		},
		{
			name:         "int response",
			functionName: "IntResponse",
			detail:       "response",
			source: `package fixture

type Request struct{}

// @service:Fixture
func IntResponse(req Request) int { return 0 }
`,
		},
		{
			name:         "bool request",
			functionName: "BoolRequest",
			detail:       "request",
			source: `package fixture

type Response struct{}

// @service:Fixture
func BoolRequest(req bool) Response { return Response{} }
`,
		},
		{
			name:         "primitive alias",
			functionName: "AliasRequest",
			detail:       "request",
			source: `package fixture

type Alias = string
type Response struct{}

// @service:Fixture
func AliasRequest(req Alias) Response { return Response{} }
`,
		},
		{
			name:         "defined primitive",
			functionName: "DefinedRequest",
			detail:       "request",
			source: `package fixture

type Identifier string
type Response struct{}

// @service:Fixture
func DefinedRequest(req Identifier) Response { return Response{} }
`,
		},
		{
			name:         "interface request",
			functionName: "InterfaceRequest",
			detail:       "request",
			source: `package fixture

type Contract interface { Call() }
type Response struct{}

// @service:Fixture
func InterfaceRequest(req Contract) Response { return Response{} }
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGo(writeGoFixture(t, tt.source), AddParseFunc(parseDoc), AddParseStruct(parseTag))
			assertRPCSignatureError(t, err, tt.functionName, tt.detail)
		})
	}
}

func TestParseGoAcceptsValueAndPointerReceiverRPCsDeclaredBeforeMessages(t *testing.T) {
	path := writeGoFixture(t, `package fixture

import "context"

type Handler struct{}

// @service:Fixture
// @method:get
// @router:/value
func (Handler) Value(req Request) Response { return Response{} }

// @service:Fixture
// @method:post
// @router:/pointer
func (*Handler) Pointer(ctx context.Context, req *Request) (*Response, error) { return nil, nil }

type Request struct{}
type Response struct{}
`)

	parsed, err := ParseGo(path, AddParseFunc(parseDoc), AddParseStruct(parseTag))
	if err != nil {
		t.Fatalf("ParseGo() error = %v", err)
	}
	generated := parsed.(*GoParsePB).Generate()
	for _, signature := range []string{
		"rpc Value (Request) returns (Response)",
		"rpc Pointer (Request) returns (Response)",
	} {
		if !strings.Contains(generated, signature) {
			t.Fatalf("generated proto is missing %q:\n%s", signature, generated)
		}
	}
}

func TestParseGoRejectsRPCSignatureBoundaries(t *testing.T) {
	tests := []struct {
		name         string
		functionName string
		detail       string
		source       string
	}{
		{
			name:         "selector request",
			functionName: "SelectorRequest",
			detail:       "request",
			source: `package fixture

import "net/http"
type Response struct{}

// @service:Fixture
func SelectorRequest(req http.Request) Response { return Response{} }
`,
		},
		{
			name:         "variadic request",
			functionName: "VariadicRequest",
			detail:       "request",
			source: `package fixture

type Request struct{}
type Response struct{}

// @service:Fixture
func VariadicRequest(req ...Request) Response { return Response{} }
`,
		},
		{
			name:         "multiple requests",
			functionName: "MultipleRequests",
			detail:       "exactly one request",
			source: `package fixture

type Request struct{}
type Response struct{}

// @service:Fixture
func MultipleRequests(first Request, second Request) Response { return Response{} }
`,
		},
		{
			name:         "non-error second result",
			functionName: "SecondResult",
			detail:       "second result",
			source: `package fixture

type Request struct{}
type Response struct{}

// @service:Fixture
func SecondResult(req Request) (Response, string) { return Response{}, "" }
`,
		},
		{
			name:         "too many results",
			functionName: "TooManyResults",
			detail:       "expected one response",
			source: `package fixture

type Request struct{}
type Response struct{}

// @service:Fixture
func TooManyResults(req Request) (Response, error, error) { return Response{}, nil, nil }
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGo(writeGoFixture(t, tt.source), AddParseFunc(parseDoc), AddParseStruct(parseTag))
			assertRPCSignatureError(t, err, tt.functionName, tt.detail)
		})
	}
}

func TestParseGoRejectsShadowedErrorResults(t *testing.T) {
	tests := []struct {
		name         string
		functionName string
		source       string
	}{
		{
			name:         "package type",
			functionName: "PackageShadow",
			source: `package fixture

type error string
type Request struct{}
type Response struct{}

// @service:Fixture
func PackageShadow(req Request) (Response, error) { return Response{}, "shadowed" }
`,
		},
		{
			name:         "type parameter",
			functionName: "TypeParameterShadow",
			source: `package fixture

type Request struct{}
type Response struct{}

// @service:Fixture
func TypeParameterShadow[error any](req Request) (Response, error) {
	var shadowed error
	return Response{}, shadowed
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseGo(writeGoFixture(t, tt.source), AddParseFunc(parseDoc), AddParseStruct(parseTag))
			assertRPCSignatureError(t, err, tt.functionName, "second result")
		})
	}
}

func TestHasRPCDocTagsClassifiesMultilineLinesExactly(t *testing.T) {
	if !hasRPCDocTags([]string{`/*
 * ordinary prose
 * @router: /v1/items
 */`}) {
		t.Fatal("hasRPCDocTags() = false for an exact tag on a later block-comment line")
	}
	if hasRPCDocTags([]string{`/*
 * ordinary prose mentioning @router: /wrong
 * @router : /near-match
 */`}) {
		t.Fatal("hasRPCDocTags() = true for prose or a near-match tag")
	}
}

func TestGeneratedRPCProtoCompilesWithProtoc(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is not installed")
	}

	parsed, err := ParseGo(writeGoFixture(t, `package fixture

import "context"

type Request struct{}
type Response struct{}

// @service:Fixture
// @method:post
// @router:/compile
func Compile(ctx context.Context, req *Request) (*Response, error) { return nil, nil }
`), AddParseFunc(parseDoc), AddParseStruct(parseTag))
	if err != nil {
		t.Fatalf("ParseGo() error = %v", err)
	}

	root := t.TempDir()
	writeProtoFixture(t, root, "service.proto", parsed.(*GoParsePB).Generate())
	writeProtoFixture(t, root, "google/api/http.proto", `syntax = "proto3";
package google.api;
message HttpRule {
  string get = 2;
  string put = 3;
  string post = 4;
  string delete = 5;
  string patch = 6;
  string body = 7;
}
`)
	writeProtoFixture(t, root, "google/api/annotations.proto", `syntax = "proto3";
package google.api;
import "google/api/http.proto";
import "google/protobuf/descriptor.proto";
extend google.protobuf.MethodOptions {
  HttpRule http = 72295728;
}
`)

	args := []string{"--proto_path=" + root}
	include := filepath.Clean(filepath.Join(filepath.Dir(protoc), "..", "include"))
	if _, statErr := os.Stat(filepath.Join(include, "google/protobuf/descriptor.proto")); statErr == nil {
		args = append(args, "--proto_path="+include)
	}
	args = append(args, "--descriptor_set_out="+filepath.Join(root, "service.pb"), "service.proto")
	command := exec.Command(protoc, args...)
	if output, commandErr := command.CombinedOutput(); commandErr != nil {
		t.Fatalf("protoc failed: %v\n%s\nproto:\n%s", commandErr, output, parsed.(*GoParsePB).Generate())
	}
}

func writeProtoFixture(t *testing.T, root, name, contents string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("create proto fixture directory: %v", err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write proto fixture: %v", err)
	}
}
