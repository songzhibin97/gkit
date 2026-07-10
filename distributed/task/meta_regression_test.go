package task

import (
	"sync"
	"testing"

	json "github.com/json-iterator/go"
)

func TestMetaJSONRoundTripPreservesValuesAndRemainsWritable(t *testing.T) {
	source := NewSignature("root", "task")
	source.Meta.Set("key", "value")
	body, err := json.Marshal(source)
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var decoded Signature
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if got, ok := decoded.Meta.Get("key"); !ok || got != "value" {
		t.Fatalf("decoded Meta[key] = (%v, %t), want (value, true); JSON=%s", got, ok, body)
	}
	decoded.Meta.Set("after", "decode")
	if got, ok := decoded.Meta.Get("after"); !ok || got != "decode" {
		t.Fatalf("decoded Meta[after] = (%v, %t), want (decode, true)", got, ok)
	}
	if !decoded.Meta.safe {
		t.Fatal("decoded Meta safe = false, want MetaSafe=true normalization")
	}
}

func TestZeroValueMetaIsWritable(t *testing.T) {
	var meta Meta
	meta.Set("key", "value")
	if got, ok := meta.Get("key"); !ok || got != "value" {
		t.Fatalf("zero-value Meta[key] = (%v, %t), want (value, true)", got, ok)
	}
}

func TestNilMetaReadAndRangeAreSafeAndSetIsNoOp(t *testing.T) {
	var meta *Meta
	if got, ok := meta.Get("missing"); ok || got != nil {
		t.Fatalf("nil Meta.Get = (%v, %t), want (nil, false)", got, ok)
	}
	called := false
	meta.Range(func(string, interface{}) { called = true })
	if called {
		t.Fatal("nil Meta.Range invoked callback")
	}
	meta.Set("key", "value")
	if meta != nil {
		t.Fatal("Set on nil *Meta unexpectedly changed the nil pointer")
	}
}

func TestCopySignatureClonesRootAndNestedMeta(t *testing.T) {
	root := NewSignature("root", "task")
	success := NewSignature("success", "task")
	failure := NewSignature("failure", "task")
	chord := NewSignature("chord", "task")
	root.Meta.Set("node", "root")
	success.Meta.Set("node", "success")
	failure.Meta.Set("node", "failure")
	chord.Meta.Set("node", "chord")
	root.CallbackOnSuccess = []*Signature{success}
	root.CallbackOnError = []*Signature{failure}
	root.CallbackChord = chord

	cloned := CopySignature(root)
	checks := []struct {
		name string
		meta *Meta
		want string
	}{
		{"root", cloned.Meta, "root"},
		{"success", cloned.CallbackOnSuccess[0].Meta, "success"},
		{"failure", cloned.CallbackOnError[0].Meta, "failure"},
		{"chord", cloned.CallbackChord.Meta, "chord"},
	}
	for _, check := range checks {
		if got, ok := check.meta.Get("node"); !ok || got != check.want {
			t.Fatalf("cloned %s Meta[node] = (%v, %t), want (%s, true)", check.name, got, ok, check.want)
		}
		if !check.meta.safe {
			t.Fatalf("cloned %s Meta safe = false", check.name)
		}
		check.meta.Set("writable", check.name)
	}

	cloned.Meta.Set("node", "clone")
	if got, _ := root.Meta.Get("node"); got != "root" {
		t.Fatalf("source root Meta changed through clone: %v", got)
	}
	root.Meta.Set("source-only", true)
	if _, ok := cloned.Meta.Get("source-only"); ok {
		t.Fatal("clone Meta shares the source map")
	}
}

func TestCopySignaturePreservesCallbackCycles(t *testing.T) {
	root := NewSignature("root", "task")
	root.Meta.Set("key", "value")
	root.CallbackOnSuccess = []*Signature{root}
	root.CallbackOnError = []*Signature{root}
	root.CallbackChord = root

	cloned := CopySignature(root)
	if cloned == nil {
		t.Fatal("CopySignature returned nil")
	}
	if cloned.CallbackOnSuccess[0] != cloned || cloned.CallbackOnError[0] != cloned || cloned.CallbackChord != cloned {
		t.Fatal("CopySignature did not preserve the root callback cycle topology")
	}
	if got, ok := cloned.Meta.Get("key"); !ok || got != "value" {
		t.Fatalf("cycle clone Meta[key] = (%v, %t), want (value, true)", got, ok)
	}
}

func TestCopySignatureNilCompatibility(t *testing.T) {
	cloned := CopySignature(nil)
	if cloned == nil {
		t.Fatal("CopySignature(nil) returned nil; want the existing non-nil empty signature behavior")
	}
}

func TestCopiedSafeMetaSupportsConcurrentAccess(t *testing.T) {
	source := NewSignature("root", "task")
	cloned := CopySignature(source)
	if cloned.Meta == nil || !cloned.Meta.safe {
		t.Fatal("cloned Meta is not initialized in safe mode")
	}

	var wg sync.WaitGroup
	for worker := 0; worker < 8; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			for iteration := 0; iteration < 100; iteration++ {
				cloned.Meta.Set("shared", worker*100+iteration)
				_, _ = cloned.Meta.Get("shared")
				cloned.Meta.Range(func(string, interface{}) {})
			}
		}()
	}
	wg.Wait()
}
