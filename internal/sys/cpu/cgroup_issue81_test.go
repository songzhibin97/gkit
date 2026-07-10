package cpu

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeIssue81CGroupFile(t *testing.T, name, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatalf("create cgroup fixture directory: %v", err)
	}
	if err := os.WriteFile(name, []byte(data), 0644); err != nil {
		t.Fatalf("write cgroup fixture %s: %v", name, err)
	}
}

func TestIssue81ParseCGroupV2(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "tenant.slice")
	writeIssue81CGroupFile(t, filepath.Join(dir, "cpu.max"), "250000 100000\n")
	writeIssue81CGroupFile(t, filepath.Join(dir, "cpu.stat"), "user_usec 100\nusage_usec 123456\nsystem_usec 200\n")
	writeIssue81CGroupFile(t, filepath.Join(dir, "cpuset.cpus.effective"), "\n")
	writeIssue81CGroupFile(t, filepath.Join(dir, "cpuset.cpus"), "1-2,4\n")

	cg, err := parseCGroup(strings.NewReader("0::/tenant.slice\n"), root)
	if err != nil {
		t.Fatalf("parse v2 cgroup: %v", err)
	}
	if !cg.unified {
		t.Fatal("v2 cgroup was not marked unified")
	}
	if got, err := cg.CPUCFSQuotaUs(); err != nil || got != 250000 {
		t.Fatalf("v2 quota = %d, %v; want 250000, nil", got, err)
	}
	if got, err := cg.CPUCFSPeriodUs(); err != nil || got != 100000 {
		t.Fatalf("v2 period = %d, %v; want 100000, nil", got, err)
	}
	if got, err := cg.CPUAcctUsage(); err != nil || got != 123456000 {
		t.Fatalf("v2 usage = %d, %v; want 123456000, nil", got, err)
	}
	if got, err := cg.CPUSetCPUs(); err != nil || len(got) != 3 {
		t.Fatalf("v2 cpuset = %v, %v; want three fallback CPUs", got, err)
	}
	writeIssue81CGroupFile(t, filepath.Join(dir, "cpuset.cpus.effective"), "0,3\n")
	if got, err := cg.CPUSetCPUs(); err != nil || !containsIssue81CPU(got, 0) || !containsIssue81CPU(got, 3) || len(got) != 2 {
		t.Fatalf("v2 effective cpuset = %v, %v; want CPUs 0 and 3", got, err)
	}

	writeIssue81CGroupFile(t, filepath.Join(dir, "cpu.max"), "max 100000\n")
	if got, err := cg.CPUCFSQuotaUs(); err != nil || got != -1 {
		t.Fatalf("unlimited v2 quota = %d, %v; want -1, nil", got, err)
	}
}

func containsIssue81CPU(cpus []uint64, want uint64) bool {
	for _, cpu := range cpus {
		if cpu == want {
			return true
		}
	}
	return false
}

func TestIssue81ParseCGroupV1Paths(t *testing.T) {
	root := t.TempDir()
	cg, err := parseCGroup(strings.NewReader("2:cpuacct,cpu:/scope\n3:cpuset:/scope\n"), root)
	if err != nil {
		t.Fatalf("parse v1 cgroup: %v", err)
	}
	wantCPU := filepath.Join(root, "cpu")
	if got := cg.cGroupSet["cpu"]; got != wantCPU {
		t.Fatalf("v1 cpu path = %q, want %q", got, wantCPU)
	}
	wantCPUAcct := filepath.Join(root, "cpuacct")
	if got := cg.cGroupSet["cpuacct"]; got != wantCPUAcct {
		t.Fatalf("v1 cpuacct path = %q, want %q", got, wantCPUAcct)
	}
	wantCPUSet := filepath.Join(root, "cpuset")
	if got := cg.cGroupSet["cpuset"]; got != wantCPUSet {
		t.Fatalf("v1 cpuset path = %q, want %q", got, wantCPUSet)
	}
}

func TestIssue81CalculateCGroupCPUUsageGuards(t *testing.T) {
	tests := []struct {
		name                     string
		total, preTotal          uint64
		system, preSystem, cores uint64
		quota                    float64
		want                     uint64
	}{
		{name: "normal", total: 1200, preTotal: 1000, system: 3000, preSystem: 2000, cores: 4, quota: 2, want: 400},
		{name: "zero quota", total: 1200, preTotal: 1000, system: 3000, preSystem: 2000, cores: 4, quota: 0, want: 0},
		{name: "unchanged system", total: 1200, preTotal: 1000, system: 2000, preSystem: 2000, cores: 4, quota: 2, want: 0},
		{name: "reset system", total: 1200, preTotal: 1000, system: 1999, preSystem: 2000, cores: 4, quota: 2, want: 0},
		{name: "reset total", total: 999, preTotal: 1000, system: 3000, preSystem: 2000, cores: 4, quota: 2, want: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := calculateCGroupCPUUsage(tt.total, tt.preTotal, tt.system, tt.preSystem, tt.cores, tt.quota); got != tt.want {
				t.Fatalf("usage = %d, want %d", got, tt.want)
			}
		})
	}
}
