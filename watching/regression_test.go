package watching

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

func TestTriggeredDumpLogUsesUniformFormat(t *testing.T) {
	type profileCase struct {
		name     string
		dumpType configureType
		action   string
		max      int
		previous func(*Watching) interface{}
		trigger  func(*Watching)
	}
	const (
		triggerMin     = 11
		triggerDiff    = 22
		triggerAbs     = 33
		previousValue  = 44
		currentValue   = 55
		goroutineLimit = 66
	)
	config := typeConfig{Enable: true, TriggerMin: triggerMin, TriggerDiff: triggerDiff, TriggerAbs: triggerAbs}
	tests := []profileCase{
		{
			name:     "goroutine",
			dumpType: goroutine,
			action:   "pprof",
			max:      goroutineLimit,
			previous: func(w *Watching) interface{} { return w.grNumStats.data },
			trigger: func(w *Watching) {
				w.goroutineProfile(currentValue, groupConfigs{typeConfig: &config, GoroutineTriggerNumMax: goroutineLimit})
			},
		},
		{
			name:     "memory",
			dumpType: mem,
			action:   "pprof",
			previous: func(w *Watching) interface{} { return w.memStats.data },
			trigger:  func(w *Watching) { w.memProfile(currentValue, config) },
		},
		{
			name:     "thread",
			dumpType: thread,
			action:   "pprof",
			previous: func(w *Watching) interface{} { return w.threadStats },
			trigger:  func(w *Watching) { w.threadProfile(currentValue, config) },
		},
		{
			name:     "cpu",
			dumpType: cpu,
			action:   "pprof dump",
			previous: func(w *Watching) interface{} { return w.cpuStats.data },
			trigger: func(w *Watching) {
				// Fail file creation after the trigger log, avoiding the real CPU
				// sampling sleep while still exercising cpuProfile's call site.
				w.config.DumpPath = filepath.Join(w.config.Logger.Name(), "not-a-directory")
				w.cpuProfile(currentValue, config)
			},
		},
		{
			name:     "gc heap",
			dumpType: gcHeap,
			action:   "pprof",
			previous: func(w *Watching) interface{} { return w.gcHeapStats },
			trigger:  func(w *Watching) { w.gcHeapProfile(currentValue, true, config) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "watching.log")
			w := NewWatching(WithDumpPath(dir, "watching.log"), WithTextDump())
			t.Cleanup(func() { w.setLogger(os.Stdout) })
			w.grNumStats = newRing(1)
			w.memStats = newRing(1)
			w.threadStats = newRing(1)
			w.cpuStats = newRing(1)
			w.gcHeapStats = newRing(1)
			w.grNumStats.push(previousValue)
			w.memStats.push(previousValue)
			w.threadStats.push(previousValue)
			w.cpuStats.push(previousValue)
			w.gcHeapStats.push(previousValue)
			tt.trigger(w)

			content, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			logText := strings.TrimSpace(string(content))
			if logText == "" {
				t.Fatal("triggered dump emitted no log lines")
			}
			lines := strings.Split(logText, "\n")
			first := lines[0]
			start := strings.Index(first, "[Watching]")
			if start == -1 {
				t.Fatalf("first trigger log has no UniformLogFormat payload: %q", first)
			}
			first = first[start:]
			want := fmt.Sprintf(
				UniformLogFormat,
				tt.action,
				type2name[tt.dumpType],
				triggerMin,
				triggerDiff,
				triggerAbs,
				tt.max,
				tt.previous(w),
				currentValue,
			)
			if first != want {
				t.Fatalf("first trigger log = %q, want %q; later duplicate logs must not satisfy this assertion", first, want)
			}
		})
	}
}

func TestWriteFilePreservesPercentText(t *testing.T) {
	var profile bytes.Buffer
	profile.WriteString("profile: 50% of total\nnext: %s is literal")
	err := writeFile(profile, goroutine, &DumpConfigs{
		DumpProfileType: textDump,
		DumpFullStack:   true,
	}, "")
	if err == nil {
		t.Fatal("writeFile text mode returned nil error")
	}
	if got, want := err.Error(), profile.String(); got != want {
		t.Fatalf("writeFile text = %q, want literal %q", got, want)
	}
}

func TestTrimResultKeepsAllSegmentsUpToLimit(t *testing.T) {
	makeProfile := func(count int) string {
		segments := make([]string, count)
		for index := range segments {
			segments[index] = "segment-" + string(rune('A'+index))
		}
		return strings.Join(segments, "\n\n")
	}
	for _, count := range []int{1, 3, TrimResultTopN} {
		input := makeProfile(count)
		var buffer bytes.Buffer
		buffer.WriteString(input)
		if got := trimResult(buffer); got != input {
			t.Fatalf("trimResult(%d segments) = %q, want all %q", count, got, input)
		}
	}

	input := makeProfile(TrimResultTopN + 1)
	var buffer bytes.Buffer
	buffer.WriteString(input)
	want := strings.Join(strings.Split(input, "\n\n")[:TrimResultTopN], "\n\n")
	if got := trimResult(buffer); got != want {
		t.Fatalf("trimResult(over limit) = %q, want %q", got, want)
	}
}

func TestDumpEnableSettersSynchronizeWithReaders(t *testing.T) {
	w := NewWatching()
	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for iteration := 0; iteration < 500; iteration++ {
				if (worker+iteration)%2 == 0 {
					w.EnableThreadDump().EnableGoroutineDump().EnableCPUDump().EnableMemDump().EnableGCHeapDump()
				} else {
					w.DisableThreadDump().DisableGoroutineDump().DisableCPUDump().DisableMemDump().DisableGCHeapDump()
				}
				_ = w.config.GetThreadConfigs().Enable
				_ = w.config.GetGroupConfigs().Enable
				_ = w.config.GetCPUConfigs().Enable
				_ = w.config.GetMemConfigs().Enable
				_ = w.config.GetGcHeapConfigs().Enable
			}
		}(worker)
	}
	wg.Wait()
}
