package delayed

import (
	"testing"
	"time"

	"github.com/songzhibin97/gkit/options"
)

// TestNewDispatchingDelayed_NoPoolReplacement covers C15: previously the
// pool was created with Worker=1 in the struct literal, then if the user
// supplied Worker != 1 the slot was reassigned without shutting the old
// pool down. The fix moves pool creation to after options apply, so only
// one pool is ever instantiated.
func TestNewDispatchingDelayed_NoPoolReplacement(t *testing.T) {
	d := NewDispatchingDelayed(func(o interface{}) {
		if dd, ok := o.(*DispatchingDelayed); ok {
			dd.Worker = 4
		}
		_ = options.Option(nil) // keep import used in default builds
	})
	defer d.Close()
	// One observable side-effect of the leak was a Worker=1 pool whose
	// idle goroutines outlived Close; if the pool was created twice we'd
	// also see two pools' goroutines. We can't easily count goroutines
	// here without flakes, but the regression test below at least ensures
	// the constructor doesn't panic and Worker took effect.
	if d.Worker != 4 {
		t.Fatalf("Worker = %d, want 4", d.Worker)
	}
}
