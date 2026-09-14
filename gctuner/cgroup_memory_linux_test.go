//go:build linux

package gctuner

import (
	"os"
	"runtime/debug"
	"testing"
)

// Run explicitly in an isolated cgroup v2 container with --memory=512m.
func TestCGroupMemoryLimitLinux(t *testing.T) {
	if os.Getenv("GKIT_TEST_CGROUP_MEMORY_512M") != "1" {
		t.Skip("requires an explicitly configured 512 MiB cgroup v2 container")
	}
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err != nil {
		t.Fatalf("require cgroup v2: %v", err)
	}
	got, err := getCGroupMemoryLimit()
	if err != nil {
		t.Fatal(err)
	}
	if got != 536870912 {
		t.Fatalf("memory limit=%d, want 536870912", got)
	}
	originalGC := debug.SetGCPercent(-1)
	t.Cleanup(func() { Tuning(0); debug.SetGCPercent(originalGC) })
	TuningWithAuto(true)
	tuningMu.Lock()
	defer tuningMu.Unlock()
	if globalTuner == nil {
		t.Fatal("TuningWithAuto did not install a tuner")
	}
	if threshold := globalTuner.getThreshold(); threshold != 375809638 {
		t.Fatalf("threshold=%d, want 375809638", threshold)
	}
	t.Logf("cgroup v2 memory limit=%d; installed threshold=%d", got, globalTuner.getThreshold())
}
