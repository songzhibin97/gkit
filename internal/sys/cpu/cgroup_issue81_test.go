package cpu

import (
	"fmt"
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

func issue81EscapeMountInfoPath(name string) string {
	return strings.NewReplacer(
		"\\", "\\134",
		" ", "\\040",
		"\t", "\\011",
		"\n", "\\012",
	).Replace(name)
}

func issue81MountInfoLine(id int, root, mountPoint, fsType, options string) string {
	return fmt.Sprintf("%d 1 0:%d %s %s rw,nosuid,nodev,noexec - %s cgroup %s\n",
		id, id, issue81EscapeMountInfoPath(root), issue81EscapeMountInfoPath(mountPoint), fsType, options)
}

func TestIssue81ParseCGroupV2(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "tenant.slice")
	writeIssue81CGroupFile(t, filepath.Join(root, "cpu.max"), "max 100000\n")
	writeIssue81CGroupFile(t, filepath.Join(dir, "cpu.max"), "250000 100000\n")
	writeIssue81CGroupFile(t, filepath.Join(dir, "cpu.stat"), "user_usec 100\nusage_usec 123456\nsystem_usec 200\n")
	writeIssue81CGroupFile(t, filepath.Join(dir, "cpuset.cpus.effective"), "\n")
	writeIssue81CGroupFile(t, filepath.Join(dir, "cpuset.cpus"), "1-2,4\n")

	mountInfo := issue81MountInfoLine(21, "/", root, "cgroup2", "rw")
	cg, err := parseCGroup(strings.NewReader("0::/tenant.slice\n"), strings.NewReader(mountInfo))
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
	cpuMount := filepath.Join(root, "cpu-bind")
	cpusetMount := filepath.Join(root, "cpuset")
	mountInfo := issue81MountInfoLine(31, "/docker/containers", cpuMount, "cgroup", "rw,cpu,cpuacct") +
		issue81MountInfoLine(32, "/", cpusetMount, "cgroup", "rw,cpuset")
	cg, err := parseCGroup(
		strings.NewReader("2:cpuacct,cpu:/docker/containers/scope\n3:cpuset:/scope\n"),
		strings.NewReader(mountInfo),
	)
	if err != nil {
		t.Fatalf("parse v1 cgroup: %v", err)
	}
	wantCPU := filepath.Join(cpuMount, "scope")
	if got := cg.cGroupSet["cpu"]; got != wantCPU {
		t.Fatalf("v1 cpu path = %q, want %q", got, wantCPU)
	}
	wantCPUAcct := filepath.Join(cpuMount, "scope")
	if got := cg.cGroupSet["cpuacct"]; got != wantCPUAcct {
		t.Fatalf("v1 cpuacct path = %q, want %q", got, wantCPUAcct)
	}
	wantCPUSet := filepath.Join(cpusetMount, "scope")
	if got := cg.cGroupSet["cpuset"]; got != wantCPUSet {
		t.Fatalf("v1 cpuset path = %q, want %q", got, wantCPUSet)
	}
}

func TestIssue81HybridUsesV1CPUController(t *testing.T) {
	root := t.TempDir()
	v1Mount := filepath.Join(root, "v1-cpu")
	v2Mount := filepath.Join(root, "unified")
	v1CPU := filepath.Join(v1Mount, "v1.scope")
	writeIssue81CGroupFile(t, filepath.Join(v1CPU, "cpu.cfs_quota_us"), "100000\n")
	writeIssue81CGroupFile(t, filepath.Join(v1CPU, "cpu.cfs_period_us"), "100000\n")
	writeIssue81CGroupFile(t, filepath.Join(v2Mount, "v2.scope", "cpu.max"), "900000 100000\n")

	mountInfo := issue81MountInfoLine(41, "/", v2Mount, "cgroup2", "rw") +
		issue81MountInfoLine(42, "/", v1Mount, "cgroup", "rw,cpu,cpuacct")
	cg, err := parseCGroup(
		strings.NewReader("0::/v2.scope\n2:cpu,cpuacct:/v1.scope\n"),
		strings.NewReader(mountInfo),
	)
	if err != nil {
		t.Fatalf("parse hybrid cgroup: %v", err)
	}
	if cg.unified {
		t.Fatal("hybrid cgroup used unified hierarchy despite a v1 cpu membership")
	}
	if got, err := cg.CPUCFSQuotaUs(); err != nil || got != 100000 {
		t.Fatalf("hybrid quota = %d, %v; want v1 quota 100000, nil", got, err)
	}
}

func TestIssue81HybridDoesNotFallbackFromHiddenV1CPU(t *testing.T) {
	v2Mount := t.TempDir()
	mountInfo := issue81MountInfoLine(43, "/", v2Mount, "cgroup2", "rw")
	_, err := parseCGroup(
		strings.NewReader("0::/v2.scope\n2:cpu,cpuacct:/v1.scope\n"),
		strings.NewReader(mountInfo),
	)
	if err == nil {
		t.Fatal("hidden v1 cpu hierarchy silently fell back to cgroup v2")
	}
}

func TestIssue81V2QuotaIncludesFiniteParent(t *testing.T) {
	outer := t.TempDir()
	root := filepath.Join(outer, "visible-root")
	parent := filepath.Join(root, "tenant.slice")
	child := filepath.Join(parent, "workload.scope")
	writeIssue81CGroupFile(t, filepath.Join(outer, "cpu.max"), "1 100000\n")
	writeIssue81CGroupFile(t, filepath.Join(root, "cpu.max"), "100000 25000\n")
	writeIssue81CGroupFile(t, filepath.Join(parent, "cpu.max"), "150000 100000\n")
	writeIssue81CGroupFile(t, filepath.Join(child, "cpu.max"), "max 100000\n")

	mountInfo := issue81MountInfoLine(51, "/", root, "cgroup2", "rw")
	cg, err := parseCGroup(
		strings.NewReader("0::/tenant.slice/workload.scope\n"),
		strings.NewReader(mountInfo),
	)
	if err != nil {
		t.Fatalf("parse nested v2 cgroup: %v", err)
	}
	if got, err := cg.CPUCFSQuotaUs(); err != nil || got != 150000 {
		t.Fatalf("effective v2 quota = %d, %v; want parent quota 150000, nil", got, err)
	}
	if got, err := cg.CPUCFSPeriodUs(); err != nil || got != 100000 {
		t.Fatalf("effective v2 period = %d, %v; want parent period 100000, nil", got, err)
	}
}

func TestIssue81MountInfoEscapedRootAndMountPoint(t *testing.T) {
	outer := t.TempDir()
	mountPoint := filepath.Join(outer, "visible\tcgroup")
	dir := filepath.Join(mountPoint, "workload scope")
	writeIssue81CGroupFile(t, filepath.Join(mountPoint, "cpu.max"), "max 100000\n")
	writeIssue81CGroupFile(t, filepath.Join(dir, "cpu.max"), "250000 100000\n")

	mountInfo := issue81MountInfoLine(61, "/tenant root", mountPoint, "cgroup2", "rw")
	cg, err := parseCGroup(
		strings.NewReader("0::/tenant root/workload scope\n"),
		strings.NewReader(mountInfo),
	)
	if err != nil {
		t.Fatalf("parse escaped mountinfo: %v", err)
	}
	if got := cg.cGroupSet[""]; got != dir {
		t.Fatalf("escaped mount path = %q, want %q", got, dir)
	}
	if got, err := cg.CPUCFSQuotaUs(); err != nil || got != 250000 {
		t.Fatalf("escaped mount quota = %d, %v; want 250000, nil", got, err)
	}
}

func TestIssue81MountInfoPathEscapes(t *testing.T) {
	got, err := unescapeMountInfoPath(`/tenant\040root/tab\011line\012slash\134name`)
	if err != nil {
		t.Fatalf("unescape mountinfo path: %v", err)
	}
	want := "/tenant root/tab\tline\nslash\\name"
	if got != want {
		t.Fatalf("unescaped mountinfo path = %q, want %q", got, want)
	}
}

func TestIssue81V2QuotaPropagatesMissingAncestor(t *testing.T) {
	root := t.TempDir()
	child := filepath.Join(root, "workload.scope")
	writeIssue81CGroupFile(t, filepath.Join(child, "cpu.max"), "max 100000\n")

	mountInfo := issue81MountInfoLine(71, "/", root, "cgroup2", "rw")
	cg, err := parseCGroup(
		strings.NewReader("0::/workload.scope\n"),
		strings.NewReader(mountInfo),
	)
	if err != nil {
		t.Fatalf("parse nested v2 cgroup: %v", err)
	}
	if _, err := cg.CPUCFSQuotaUs(); err == nil {
		t.Fatal("missing ancestor cpu.max was silently ignored")
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
