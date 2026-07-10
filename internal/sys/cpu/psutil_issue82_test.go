package cpu

import (
	"testing"

	psutil "github.com/shirou/gopsutil/cpu"
)

func TestBuildCPUInfoHandlesEmptyStats(t *testing.T) {
	if got := buildCPUInfo(nil, 8); got != (Info{}) {
		t.Fatalf("buildCPUInfo(nil) = %#v, want zero Info", got)
	}
	got := buildCPUInfo([]psutil.InfoStat{{Mhz: 2400}}, 8)
	if got.Frequency != 2400 || got.Quota != 8 {
		t.Fatalf("buildCPUInfo() = %#v, want frequency=2400 quota=8", got)
	}
}

func TestPercentToUsageHandlesEmptySamples(t *testing.T) {
	if got := percentToUsage(nil); got != 0 {
		t.Fatalf("percentToUsage(nil) = %d, want 0", got)
	}
	if got := percentToUsage([]float64{}); got != 0 {
		t.Fatalf("percentToUsage(empty) = %d, want 0", got)
	}
	if got := percentToUsage([]float64{12.5}); got != 125 {
		t.Fatalf("percentToUsage(12.5) = %d, want 125", got)
	}
}
