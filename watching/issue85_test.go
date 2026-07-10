package watching

import (
	"bytes"
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
		trigger  func(*Watching)
	}
	config := typeConfig{Enable: true, TriggerMin: 0, TriggerDiff: 0, TriggerAbs: 0}
	tests := []profileCase{
		{
			name:     "goroutine",
			dumpType: goroutine,
			action:   "pprof",
			trigger: func(w *Watching) {
				w.goroutineProfile(1, groupConfigs{typeConfig: &config, GoroutineTriggerNumMax: 100})
			},
		},
		{name: "memory", dumpType: mem, action: "pprof", trigger: func(w *Watching) { w.memProfile(1, config) }},
		{name: "thread", dumpType: thread, action: "pprof", trigger: func(w *Watching) { w.threadProfile(1, config) }},
		{
			name:     "cpu",
			dumpType: cpu,
			action:   "pprof dump",
			trigger: func(w *Watching) {
				// Fail file creation after the trigger log, avoiding the real CPU
				// sampling sleep while still exercising cpuProfile's call site.
				w.config.DumpPath = filepath.Join(w.config.Logger.Name(), "not-a-directory")
				w.cpuProfile(1, config)
			},
		},
		{name: "gc heap", dumpType: gcHeap, action: "pprof", trigger: func(w *Watching) { w.gcHeapProfile(1, true, config) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			logPath := filepath.Join(dir, "watching.log")
			w := NewWatching(WithDumpPath(dir, "watching.log"), WithTextDump())
			t.Cleanup(func() { w.setLogger(os.Stdout) })
			tt.trigger(w)

			content, err := os.ReadFile(logPath)
			if err != nil {
				t.Fatal(err)
			}
			logText := string(content)
			if strings.Contains(logText, "%!(EXTRA") {
				t.Fatalf("triggered dump log treated its label as a format string: %q", logText)
			}
			want := "[Watching] " + tt.action + " " + type2name[tt.dumpType]
			if !strings.Contains(logText, want) {
				t.Fatalf("triggered dump log lacks %q: %q", want, logText)
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
