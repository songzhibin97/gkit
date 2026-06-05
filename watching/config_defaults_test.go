package watching

import "testing"

// TestDefaultConfigs_FieldsAreNotPermuted covers C-defaults: each of the
// five default config constructors previously assigned trigger Min/Abs/Diff
// from the WRONG `default*` constants — Min ← *Abs, Abs ← *Diff,
// Diff ← *Min — so any default-config user observed garbage thresholds.
func TestDefaultConfigs_FieldsAreNotPermuted(t *testing.T) {
	cases := []struct {
		name string
		got  *typeConfig
		min  int
		abs  int
		diff int
	}{
		{"goroutine", defaultGroupConfigs().typeConfig, defaultGoroutineTriggerMin, defaultGoroutineTriggerAbs, defaultGoroutineTriggerDiff},
		{"mem", defaultMemConfigs(), defaultMemTriggerMin, defaultMemTriggerAbs, defaultMemTriggerDiff},
		{"cpu", defaultCPUConfigs(), defaultCPUTriggerMin, defaultCPUTriggerAbs, defaultCPUTriggerDiff},
		{"thread", defaultThreadConfig(), defaultThreadTriggerMin, defaultThreadTriggerAbs, defaultThreadTriggerDiff},
		{"gcheap", defaultGCHeapOptions(), defaultGCHeapTriggerMin, defaultGCHeapTriggerAbs, defaultGCHeapTriggerDiff},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if c.got.TriggerMin != c.min {
				t.Errorf("%s TriggerMin = %d, want %d", c.name, c.got.TriggerMin, c.min)
			}
			if c.got.TriggerAbs != c.abs {
				t.Errorf("%s TriggerAbs = %d, want %d", c.name, c.got.TriggerAbs, c.abs)
			}
			if c.got.TriggerDiff != c.diff {
				t.Errorf("%s TriggerDiff = %d, want %d", c.name, c.got.TriggerDiff, c.diff)
			}
		})
	}
}
