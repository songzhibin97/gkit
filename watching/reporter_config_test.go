package watching

import (
	"sync"
	"testing"
)

func TestReporterConfigSnapshotPreservesSwitchAndReporter(t *testing.T) {
	reporter := &unusedProfileReporter{}
	w := NewWatching(WithProfileReporter(reporter))
	first := w.config.GetReporterConfigs()
	if first.reporter != reporter || first.active != 1 {
		t.Fatalf("initial snapshot = %+v", first)
	}
	w.DisableProfileReporter()
	disabled := w.config.GetReporterConfigs()
	if disabled.reporter != reporter || disabled.active != 0 {
		t.Fatalf("disabled snapshot = %+v", disabled)
	}
	if first.active != 1 {
		t.Fatal("earlier snapshot changed after disabling")
	}
	w.EnableProfileReporter()
	enabled := w.config.GetReporterConfigs()
	if enabled.reporter != reporter || enabled.active != 1 {
		t.Fatalf("enabled snapshot = %+v", enabled)
	}
	if disabled.active != 0 {
		t.Fatal("earlier snapshot changed after enabling")
	}
	empty := NewWatching(WithLoggerLevel(-1))
	empty.EnableProfileReporter()
	if got := empty.config.GetReporterConfigs(); got.reporter != nil || got.active != 0 {
		t.Fatalf("nil reporter was activated: %+v", got)
	}
}

func TestReporterSwitchConcurrentSnapshots(t *testing.T) {
	reporter := &unusedProfileReporter{}
	w := NewWatching(WithProfileReporter(reporter))
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 2000; i++ {
			w.DisableProfileReporter()
			w.EnableProfileReporter()
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 2000; i++ {
			got := w.config.GetReporterConfigs()
			if got.reporter != reporter || (got.active != 0 && got.active != 1) {
				t.Errorf("invalid snapshot = %+v", got)
				return
			}
		}
	}()
	close(start)
	wg.Wait()
	if got := w.config.GetReporterConfigs(); got.reporter != reporter || got.active != 1 {
		t.Fatalf("final snapshot = %+v", got)
	}
}
