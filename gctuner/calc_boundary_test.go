package gctuner

import "testing"

func TestCalcGCPercentWideClamp(t *testing.T) {
	oldMin, oldMax := SetMinGCPercent(50), SetMaxGCPercent(500)
	t.Cleanup(func() { SetMinGCPercent(oldMin); SetMaxGCPercent(oldMax) })
	for _, tt := range []struct {
		name             string
		inuse, threshold uint64
		want             uint32
	}{
		{"uint32 wrap", 25, 1073741849, 500},
		{"page aligned wrap", 26214400, 1125899933057024, 500},
		{"uint64 maximum", 1, ^uint64(0), 500},
		{"exact minimum", 100, 150, 50},
		{"exact maximum", 100, 600, 500},
		{"floor", 100, 266, 166},
		{"below minimum", 100, 149, 50},
		{"above maximum", 100, 601, 500},
		{"equal threshold", 100, 100, 50},
		{"below inuse", 100, 99, 50},
		{"zero inuse", 0, 100, defaultGCPercent},
		{"zero threshold", 100, 0, defaultGCPercent},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := calcGCPercent(tt.inuse, tt.threshold); got != tt.want {
				t.Fatalf("calcGCPercent(%d,%d)=%d, want %d", tt.inuse, tt.threshold, got, tt.want)
			}
		})
	}
}
