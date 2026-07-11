package parse_go

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGoParsePB_GeneratePB(t *testing.T) {
	path := writeBehaviorFixture(t, `package fixture

type Request struct {
	Name string
}

type Response struct {
	Message string
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
	for _, want := range []string{
		"package fixture;",
		"message Request{",
		"string Name = 1;",
		"message Response{",
		"string Message = 1;",
		"service User{",
		"rpc Register (Request) returns (Response)",
		`post : "/register"`,
	} {
		if !strings.Contains(generated, want) {
			t.Fatalf("Generate() is missing %q:\n%s", want, generated)
		}
	}
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
