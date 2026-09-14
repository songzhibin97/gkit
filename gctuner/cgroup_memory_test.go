package gctuner

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func memoryFixture(t *testing.T, name, data string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte(data), 0644); err != nil {
		t.Fatal(err)
	}
}

func memoryMount(root, mount, fsType, options string) string {
	escape := strings.NewReplacer("\\", "\\134", " ", "\\040", "\t", "\\011", "\n", "\\012")
	return fmt.Sprintf("31 1 0:31 %s %s rw - %s cgroup %s\n", escape.Replace(root), escape.Replace(mount), fsType, options)
}

func TestCGroupMemoryLimits(t *testing.T) {
	for _, tt := range []struct {
		name, fsType, leaf, parent, root string
		host, want                       uint64
	}{
		{"v2 leaf", "cgroup2", "512", "768", "max", 1024, 512},
		{"v2 parent", "cgroup2", "max", "256", "768", 1024, 256},
		{"v2 root", "cgroup2", "512", "768", "128", 1024, 128},
		{"v2 unlimited", "cgroup2", "max", "max", "max", 1024, 1024},
		{"v2 host bound", "cgroup2", "4096", "2048", "max", 1024, 1024},
		{"v2 zero", "cgroup2", "0", "max", "max", 1024, 0},
		{"v1 leaf", "cgroup", "512", "768", "9223372036854771712", 1024, 512},
		{"v1 ancestor", "cgroup", "768", "256", "9223372036854771712", 1024, 256},
		{"v1 unlimited", "cgroup", "9223372036854771712", "9223372036854771712", "9223372036854771712", 1024, 1024},
		{"v1 negative unlimited", "cgroup", "-1", "-1", "-1", 1024, 1024},
		{"integer host bound", "cgroup2", "max", "max", "max", 9007199254740993, 9007199254740993},
		{"integer cgroup bound", "cgroup2", "9007199254740993", "max", "max", 9007199254741000, 9007199254740993},
	} {
		t.Run(tt.name, func(t *testing.T) {
			outer := t.TempDir()
			mount := filepath.Join(outer, "visible memory\tmount")
			member := "0::/tenant root/workload/leaf\n"
			filename, options := "memory.max", "rw"
			if tt.fsType == "cgroup" {
				member = "2:cpu:/other\n3:cpuacct,memory:/tenant root/workload/leaf\n0::/unified\n"
				filename, options = "memory.limit_in_bytes", "rw,cpuacct,memory"
			}
			if tt.fsType == "cgroup" {
				memoryFixture(t, filepath.Join(mount, "memory.use_hierarchy"), "1")
				memoryFixture(t, filepath.Join(mount, "workload", "memory.use_hierarchy"), "1")
			}
			memoryFixture(t, filepath.Join(outer, filename), "1") // Outside the visible mount is not an ancestor we can inspect.
			memoryFixture(t, filepath.Join(mount, filename), tt.root)
			memoryFixture(t, filepath.Join(mount, "workload", filename), tt.parent)
			memoryFixture(t, filepath.Join(mount, "workload", "leaf", filename), tt.leaf+"\n")
			got, err := cgroupMemoryLimit(member, memoryMount("/tenant root", mount, tt.fsType, options), tt.host)
			if err != nil || got != tt.want {
				t.Fatalf("limit=%d, err=%v; want %d", got, err, tt.want)
			}
		})
	}
}

func TestCGroupMemoryBroadMountRetainsAncestors(t *testing.T) {
	outer, bound := t.TempDir(), t.TempDir()
	memoryFixture(t, filepath.Join(outer, "memory.max"), "max")
	memoryFixture(t, filepath.Join(outer, "tenant", "memory.max"), "256")
	memoryFixture(t, filepath.Join(outer, "tenant", "leaf", "memory.max"), "512")
	memoryFixture(t, filepath.Join(bound, "memory.max"), "512")
	for _, mounts := range []string{
		memoryMount("/tenant/leaf", bound, "cgroup2", "rw") + memoryMount("/", outer, "cgroup2", "rw"),
		memoryMount("/", outer, "cgroup2", "rw") + memoryMount("/tenant/leaf", bound, "cgroup2", "rw"),
	} {
		got, err := cgroupMemoryLimit("0::/tenant/leaf\n", mounts, 1024)
		if err != nil || got != 256 {
			t.Fatalf("limit=%d, err=%v; want ancestor 256", got, err)
		}
	}
}

func TestCGroupMemoryUnifiedRootWithoutLimit(t *testing.T) {
	mount := t.TempDir()
	memoryFixture(t, filepath.Join(mount, "cgroup.controllers"), "cpu memory\n")
	memoryFixture(t, filepath.Join(mount, "leaf", "memory.max"), "512")
	for _, tt := range []struct {
		member string
		want   uint64
	}{{"/", 1024}, {"/leaf", 512}} {
		got, err := cgroupMemoryLimit("0::"+tt.member+"\n", memoryMount("/", mount, "cgroup2", "rw"), 1024)
		if err != nil || got != tt.want {
			t.Fatalf("member=%s limit=%d, err=%v; want %d", tt.member, got, err, tt.want)
		}
	}
}

func TestCGroupMemoryReadFailures(t *testing.T) {
	for _, tt := range []struct{ name, leaf, parent string }{
		{"missing leaf", "", "max"},
		{"missing ancestor", "max", ""},
		{"invalid leaf", "garbage", "max"},
		{"invalid ancestor", "512", "bad"},
		{"negative v2", "-1", "max"},
		{"overflow", "18446744073709551616", "max"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			mount := t.TempDir()
			if tt.leaf != "" {
				memoryFixture(t, filepath.Join(mount, "child", "memory.max"), tt.leaf)
			}
			if tt.parent != "" {
				memoryFixture(t, filepath.Join(mount, "memory.max"), tt.parent)
			}
			_, err := cgroupMemoryLimit("0::/tenant/child\n", memoryMount("/tenant", mount, "cgroup2", "rw"), 1024)
			if err == nil {
				t.Fatal("expected read/parse error")
			}
		})
	}
}

func TestCGroupMemoryMembershipFailures(t *testing.T) {
	mount := t.TempDir()
	memoryFixture(t, filepath.Join(mount, "memory.max"), "max")
	for _, tt := range []struct{ name, member, mounts string }{
		{"no member", "", memoryMount("/", mount, "cgroup2", "rw")},
		{"malformed member", "0:/broken\n", memoryMount("/", mount, "cgroup2", "rw")},
		{"relative member", "0::relative\n", memoryMount("/", mount, "cgroup2", "rw")},
		{"hidden v1", "0::/\n2:memory:/hidden\n", memoryMount("/", mount, "cgroup2", "rw")},
		{"wrong controller", "2:memory:/\n", memoryMount("/", mount, "cgroup", "rw,memoryswap")},
		{"prefix mismatch", "0::/tenant-other\n", memoryMount("/tenant", mount, "cgroup2", "rw")},
		{"bad mountinfo", "0::/\n", "malformed\n"},
		{"relative mount", "0::/\n", memoryMount("/", "relative", "cgroup2", "rw")},
		{"bad escape", "0::/\n", `31 1 0:31 / /bad\999 rw - cgroup2 cgroup rw`},
		{"truncated escape", "0::/\n", `31 1 0:31 / /bad\ rw - cgroup2 cgroup rw`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got, err := cgroupMemoryLimit(tt.member, tt.mounts, 1024); err == nil {
				t.Fatalf("unexpected limit=%d without error", got)
			}
		})
	}
}

func TestCGroupMemoryV1NonHierarchicalAncestor(t *testing.T) {
	mount := t.TempDir()
	memoryFixture(t, filepath.Join(mount, "memory.limit_in_bytes"), "128")
	memoryFixture(t, filepath.Join(mount, "child", "memory.limit_in_bytes"), "512")
	mounts := memoryMount("/", mount, "cgroup", "rw,memory")
	for _, tt := range []struct {
		hierarchy string
		want      uint64
		wantErr   bool
	}{
		{"0", 512, false}, {"1", 128, false}, {"bad", 0, true},
	} {
		memoryFixture(t, filepath.Join(mount, "memory.use_hierarchy"), tt.hierarchy)
		got, err := cgroupMemoryLimit("2:memory:/child\n", mounts, 1024)
		if (err != nil) != tt.wantErr || got != tt.want {
			t.Fatalf("hierarchy=%s limit=%d, err=%v; want %d, error=%v", tt.hierarchy, got, err, tt.want, tt.wantErr)
		}
	}
	if err := os.Remove(filepath.Join(mount, "memory.use_hierarchy")); err != nil {
		t.Fatal(err)
	}
	if _, err := cgroupMemoryLimit("2:memory:/child\n", mounts, 1024); err == nil {
		t.Fatal("missing hierarchy setting was ignored")
	}
}

func TestCGroupMemoryRootRequiresMemoryController(t *testing.T) {
	for _, controllers := range []string{"", "cpu io\n", "memoryswap\n"} {
		mount := t.TempDir()
		memoryFixture(t, filepath.Join(mount, "cgroup.controllers"), controllers)
		if got, err := cgroupMemoryLimit("0::/\n", memoryMount("/", mount, "cgroup2", "rw"), 1024); err == nil {
			t.Fatalf("controllers=%q: returned limit=%d without a visible memory controller", controllers, got)
		}
	}
}

func TestCGroupMemoryV2InheritsWithoutDelegation(t *testing.T) {
	for _, depth := range []int{1, 3} {
		t.Run(fmt.Sprintf("depth-%d", depth), func(t *testing.T) {
			mount := t.TempDir()
			memoryFixture(t, filepath.Join(mount, "memory.max"), "max")
			memoryFixture(t, filepath.Join(mount, "cgroup.controllers"), "cpu memory")
			memoryFixture(t, filepath.Join(mount, "cgroup.subtree_control"), "cpu memory")
			memoryFixture(t, filepath.Join(mount, "tenant", "memory.max"), "536870912")
			memoryFixture(t, filepath.Join(mount, "tenant", "cgroup.controllers"), "cpu memory")
			memoryFixture(t, filepath.Join(mount, "tenant", "cgroup.subtree_control"), "cpu")
			member := "/tenant"
			for i := 0; i < depth; i++ {
				member += "/leaf"
				controllers := ""
				if i == 0 {
					controllers = "cpu"
				}
				memoryFixture(t, filepath.Join(mount, member, "cgroup.controllers"), controllers)
				memoryFixture(t, filepath.Join(mount, member, "cgroup.subtree_control"), "")
			}
			got, err := cgroupMemoryLimit("0::"+member+"\n", memoryMount("/", mount, "cgroup2", "rw"), 1073741824)
			if err != nil || got != 536870912 {
				t.Fatalf("inherited limit=%d, err=%v; want 536870912", got, err)
			}
		})
	}
}

func TestCGroupMemoryV2AvailableControllerRequiresLimit(t *testing.T) {
	mount := t.TempDir()
	memoryFixture(t, filepath.Join(mount, "memory.max"), "512")
	memoryFixture(t, filepath.Join(mount, "leaf", "cgroup.controllers"), "cpu memory\n")
	if got, err := cgroupMemoryLimit("0::/leaf\n", memoryMount("/", mount, "cgroup2", "rw"), 1024); err == nil {
		t.Fatalf("missing active memory.max returned %d without error", got)
	}
}

func TestCGroupMemoryV2DoesNotGuessHiddenParentLimit(t *testing.T) {
	mount := t.TempDir()
	memoryFixture(t, filepath.Join(mount, "cgroup.controllers"), "cpu\n")
	if got, err := cgroupMemoryLimit("0::/tenant/leaf\n", memoryMount("/tenant/leaf", mount, "cgroup2", "rw"), 1024); err == nil {
		t.Fatalf("hidden ancestor returned %d without error", got)
	}
}
