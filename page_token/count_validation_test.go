package page_token

import "testing"

func TestProcessPageTokensRejectsNegativeCounts(t *testing.T) {
	for _, tt := range []struct {
		name       string
		total, max int
	}{
		{"negative total", -1, 0}, {"negative max", 10, -1}, {"negative max empty total", 0, -1},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pt := NewTokenGenerate("counts", SetSalt("count-test-salt"), SetMaxElements(tt.max))
			start, end, next, err := pt.ProcessPageTokens(tt.total, 1, "")
			wantError := "the maximum number of elements must not be negative"
			if tt.total < 0 {
				wantError = "the number of elements must not be negative"
			}
			if err == nil || err.Error() != wantError || start != 0 || end != 0 || next != "" {
				t.Fatalf("range=(%d,%d), hasNext=%t, error=%v; want zero range and error", start, end, next != "", err)
			}
		})
	}
}

func TestProcessPageTokensCountBoundaries(t *testing.T) {
	for _, tt := range []struct {
		name                      string
		total, max, size, wantEnd int
		wantNext                  bool
	}{
		{"empty", 0, 0, 1, 0, false},
		{"unlimited", 10, 0, 0, 10, false},
		{"bounded", 10, 3, 0, 3, false},
		{"one page", 10, 0, 2, 2, true},
		{"positive max empty", 0, 3, 0, 0, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			pt := NewTokenGenerate("counts", SetSalt("count-test-salt"), SetMaxElements(tt.max))
			start, end, next, err := pt.ProcessPageTokens(tt.total, tt.size, "")
			if err != nil || start != 0 || end != tt.wantEnd || (next != "") != tt.wantNext {
				t.Fatalf("range=(%d,%d), hasNext=%t, error=%v", start, end, next != "", err)
			}
			if tt.wantNext {
				i, err := pt.GetIndex(next)
				if err != nil || i != end {
					t.Fatalf("next index=%d error=%v", i, err)
				}
			}
		})
	}
}
