package match

import "testing"

func TestQuestionWildcardRequiresOneRune(t *testing.T) {
	tests := []struct {
		name    string
		str     string
		pattern string
		want    bool
	}{
		{name: "ASCII exhausted", str: "ab", pattern: "ab?", want: false},
		{name: "multibyte exhausted", str: "\u4e2d", pattern: "\u4e2d?", want: false},
		{name: "ASCII present", str: "abc", pattern: "ab?", want: true},
		{name: "multibyte present", str: "\u4e2d\u6587", pattern: "\u4e2d?", want: true},
		{name: "wildcard consumes multibyte", str: "\u4e2d", pattern: "?", want: true},
		{name: "wildcard consumes replacement rune", str: "\ufffd", pattern: "?", want: true},
		{name: "literal replacement rune", str: "\ufffd", pattern: "\ufffd", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Match(tt.str, tt.pattern); got != tt.want {
				t.Fatalf("Match(%q, %q) = %v, want %v", tt.str, tt.pattern, got, tt.want)
			}
		})
	}
}
