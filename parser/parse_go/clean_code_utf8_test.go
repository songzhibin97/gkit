package parse_go

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestPileDismantlePreservesUTF8AroundRemovedLine(t *testing.T) {
	const source = `package fixture

func 示例() {
	println("保留前")
	// 删除代码
	println("保留后")
}
`
	const want = `package fixture

func 示例() {
	println("保留前")
	println("保留后")
}
`

	path := filepath.Join(t.TempDir(), "fixture.go")
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	g := &GoParsePB{FilePath: path}
	if err := g.PileDismantle("// 删除代码"); err != nil {
		t.Fatalf("PileDismantle() error = %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	if !utf8.Valid(got) {
		t.Fatalf("PileDismantle() produced invalid UTF-8: %q", got)
	}
	if string(got) != want {
		t.Fatalf("PileDismantle() result:\n%s\nwant:\n%s", got, want)
	}
	for _, adjacent := range []string{"示例", "保留前", "保留后"} {
		if !strings.Contains(string(got), adjacent) {
			t.Fatalf("PileDismantle() removed adjacent Chinese text %q", adjacent)
		}
	}
}

func TestCleanCodeRemovesUTF8LineAtBoundaries(t *testing.T) {
	tests := []struct {
		name      string
		clearCode string
		source    string
		want      string
	}{
		{
			name:      "first line",
			clearCode: "// 删除代码",
			source:    "// 删除代码\n保留中文\n",
			want:      "保留中文\n",
		},
		{
			name:      "middle line",
			clearCode: "// 删除代码",
			source:    "保留前\n\t// 删除代码\n保留后\n",
			want:      "保留前\n保留后\n",
		},
		{
			name:      "CRLF middle line",
			clearCode: "// 删除代码",
			source:    "保留前\r\n\t// 删除代码\r\n保留后\r\n",
			want:      "保留前\r\n保留后\r\n",
		},
		{
			name:      "EOF after preserved Chinese",
			clearCode: "target",
			source:    "保留中文\ntarget",
			want:      "保留中文\n",
		},
		{
			name:      "single line EOF",
			clearCode: "target",
			source:    "target",
			want:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := cleanCode(tt.clearCode, tt.source)
			if err != nil {
				t.Fatalf("cleanCode() error = %v", err)
			}
			if !utf8.Valid(got) {
				t.Fatalf("cleanCode() produced invalid UTF-8: %q", got)
			}
			if string(got) != tt.want {
				t.Fatalf("cleanCode() = %q, want %q", got, tt.want)
			}
		})
	}
}
