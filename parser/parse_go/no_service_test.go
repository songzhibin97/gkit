package parse_go

import (
	"strings"
	"testing"
)

func TestGenerateProtoWithoutRPCService(t *testing.T) {
	for _, source := range []string{
		"package fixture\ntype Item struct { Name string }\n",
		"package fixture\ntype Item struct { Name string }\nfunc helper() {}\n",
	} {
		parsed, err := ParseGo(writeGoFixture(t, source))
		if err != nil {
			t.Fatal(err)
		}
		generated := parsed.Generate()
		compileGeneratedProto(t, generated)
		if strings.Contains(generated, "service ") || strings.Contains(generated, "annotations.proto") {
			t.Fatalf("unexpected service/import:\n%s", generated)
		}
		if !strings.Contains(generated, "string Name = 1;") {
			t.Fatalf("message field omitted:\n%s", generated)
		}
	}
}
