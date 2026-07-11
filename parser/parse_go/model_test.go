package parse_go

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const generatedProtoGolden = `syntax = "proto3";
import "google/api/annotations.proto";
package fixture;
// message
message Request{
string Name = 1;
int32 Age = 2;
bool Active = 3;
}
message Response{
string Message = 1;
int64 Code = 2;
}
// server
service User{
rpc Register (Request) returns (Response) {
option (google.api.http) = {
post : "/register"
};
}
}`

func TestGoParsePB_GeneratePB(t *testing.T) {
	path := writeBehaviorFixture(t, `package fixture

type Request struct {
	Name   string
	Age    int32
	Active bool
}

type Response struct {
	Message string
	Code    int64
}

// @service:User
// @method:post
// @router:/register
func Register(req Request) Response { return Response{} }
`)
	rr, err := ParseGo(path, AddParseFunc(parseDoc), AddParseStruct(parseTag))
	if err != nil {
		t.Fatalf("ParseGo() error = %v", err)
	}
	generated := rr.(*GoParsePB).Generate()
	if generated == "" {
		t.Fatal("Generate() returned an empty proto")
	}
	if got := normalizeGeneratedProto(generated); got != generatedProtoGolden {
		t.Fatalf("Generate() output:\n%s\n\nwant exact normalized proto:\n%s", got, generatedProtoGolden)
	}
	compileGeneratedBehaviorProto(t, generated)
}

func TestGoParsePB_PileDismantle(t *testing.T) {
	const source = `package fixture

func Demo() {
	// marker
	var inserted = 1
	println("keep")
}
`
	const want = `package fixture

func Demo() {
	// marker
	println("keep")
}
`
	path := writeBehaviorFixture(t, source)
	rr, err := ParseGo(path)
	if err != nil {
		t.Fatalf("ParseGo() error = %v", err)
	}
	if err := rr.(*GoParsePB).PileDismantle("var inserted = 1"); err != nil {
		t.Fatalf("PileDismantle() error = %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read temporary fixture: %v", err)
	}
	if string(got) != want {
		t.Fatalf("PileDismantle() result:\n%s\nwant:\n%s", got, want)
	}
}

func TestCheckRepeatMatchesWholeTrimmedLines(t *testing.T) {
	context := "before\n\t// exact marker\nafter\n"
	if !checkRepeat("// exact marker", context) {
		t.Fatal("checkRepeat() = false for an exact trimmed line")
	}
	if checkRepeat("exact marker", context) {
		t.Fatal("checkRepeat() = true for a substring rather than the whole line")
	}
	if checkRepeat("// missing", context) {
		t.Fatal("checkRepeat() = true for a missing line")
	}
}

func writeBehaviorFixture(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "fixture.go")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write temporary fixture: %v", err)
	}
	return path
}

func normalizeGeneratedProto(generated string) string {
	lines := strings.Split(strings.ReplaceAll(generated, "\r\n", "\n"), "\n")
	normalized := make([]string, 0, len(lines))
	for _, line := range lines {
		if line = strings.TrimSpace(line); line != "" {
			normalized = append(normalized, line)
		}
	}
	return strings.Join(normalized, "\n")
}

func compileGeneratedBehaviorProto(t *testing.T, generated string) {
	t.Helper()
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc is not installed")
	}
	googleAPIRoot := findGoogleAPIProtoRoot(t)
	dir := t.TempDir()
	protoPath := filepath.Join(dir, "generated.proto")
	if err := os.WriteFile(protoPath, []byte(generated), 0o600); err != nil {
		t.Fatalf("write generated proto: %v", err)
	}
	descriptorPath := filepath.Join(dir, "generated.pb")
	args := []string{
		"--proto_path=" + dir,
		"--proto_path=" + googleAPIRoot,
	}
	protobufInclude := filepath.Clean(filepath.Join(filepath.Dir(protoc), "..", "include"))
	if info, err := os.Stat(filepath.Join(protobufInclude, "google", "protobuf", "descriptor.proto")); err == nil && !info.IsDir() {
		args = append(args, "--proto_path="+protobufInclude)
	}
	args = append(args, "--descriptor_set_out="+descriptorPath, protoPath)
	cmd := exec.Command(protoc, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc failed: %v\n%s\nGenerated proto:\n%s", err, output, generated)
	}
}

func findGoogleAPIProtoRoot(t *testing.T) string {
	t.Helper()
	home, _ := os.UserHomeDir()
	gopath := os.Getenv("GOPATH")
	if gopath == "" && home != "" {
		gopath = filepath.Join(home, "go")
	}
	patterns := []string{
		filepath.Join(os.Getenv("GKIT_GOOGLEAPIS_PROTO_PATH"), "google", "api", "annotations.proto"),
		"/opt/homebrew/lib/python*/site-packages/google/api/annotations.proto",
		"/usr/local/lib/python*/site-packages/google/api/annotations.proto",
		filepath.Join(gopath, "pkg", "mod", "github.com", "grpc-ecosystem", "grpc-gateway@*", "third_party", "googleapis", "google", "api", "annotations.proto"),
		filepath.Join(gopath, "pkg", "mod", "github.com", "go-kratos", "kratos", "v2@*", "third_party", "google", "api", "annotations.proto"),
	}
	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		if len(matches) > 0 {
			return filepath.Dir(filepath.Dir(filepath.Dir(matches[0])))
		}
	}
	t.Skip("real google/api protos are unavailable; set GKIT_GOOGLEAPIS_PROTO_PATH")
	return ""
}
