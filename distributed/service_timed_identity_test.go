package distributed

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/songzhibin97/gkit/distributed/task"
)

func newTimedIdentityTemplate(id string, success bool) *task.Signature {
	root := task.NewSignature(id, "task")
	root.Args = []task.Arg{{Type: "string", Value: "template argument"}}
	root.Meta.Set("marker", "template")
	root.CallbackOnError = []*task.Signature{task.NewSignature(id+"-error", "error")}
	root.CallbackOnError[0].CallbackOnSuccess = []*task.Signature{task.NewSignature(id+"-nested-error", "nested")}
	root.CallbackChord = task.NewSignature(id+"-chord", "chord")
	root.CallbackChord.CallbackOnError = []*task.Signature{task.NewSignature(id+"-nested-chord", "nested")}
	if success {
		root.CallbackOnSuccess = []*task.Signature{task.NewSignature(id+"-success", "success")}
		root.CallbackOnSuccess[0].CallbackOnSuccess = []*task.Signature{task.NewSignature(id+"-nested-success", "nested")}
	}
	return root
}

// #166 / 03-07: each cron fire owns a new task/callback identity graph, while
// the registered template remains unchanged even across overlapping fires.
func TestTimedTaskAndChainUseDistinctRuntimeGraphs(t *testing.T) {
	for _, kind := range []string{"task", "chain"} {
		t.Run(kind, func(t *testing.T) {
			root := newTimedIdentityTemplate("root-template", kind == "task")
			templates := []*task.Signature{root}
			wantNodes := 7
			if kind == "chain" {
				templates = append(templates, newTimedIdentityTemplate("tail-template", true))
				wantNodes = 12
			}
			before, err := json.Marshal(templates)
			if err != nil {
				t.Fatal(err)
			}
			entered, release := make(chan struct{}, 2), make(chan struct{})
			var once sync.Once
			unblock := func() { once.Do(func() { close(release) }) }
			var mu sync.Mutex
			var published []*task.Signature
			server := &Server{
				config:  &Config{ConsumeQueue: "timed-identity"},
				backend: &groupTestBackend{},
				controller: &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
					mu.Lock()
					published = append(published, task.CopySignature(signature))
					mu.Unlock()
					entered <- struct{}{}
					<-release
					return nil
				}},
				lock: timedGroupTestLocker{}, scheduler: cron.New(),
				prePublishHandler: func(signature *task.Signature) {
					signature.Args[0].Value = "runtime argument"
					signature.Meta.Set("marker", "runtime")
				},
			}
			if kind == "task" {
				err = server.RegisteredTimedTask("* * * * *", "timed", root)
			} else {
				err = server.RegisteredTimedChain("* * * * *", "timed", templates...)
			}
			if err != nil {
				t.Fatal(err)
			}
			entries := server.scheduler.Entries()
			if len(entries) != 1 {
				t.Fatalf("jobs = %d, want 1", len(entries))
			}
			var jobs sync.WaitGroup
			jobs.Add(2)
			t.Cleanup(func() { unblock(); jobs.Wait() })
			for i := 0; i < 2; i++ {
				go func() { defer jobs.Done(); entries[0].Job.Run() }()
			}
			for i := 0; i < 2; i++ {
				select {
				case <-entered:
				case <-time.After(time.Second):
					t.Fatal("cron fire did not reach Publish")
				}
			}
			unblock()
			jobs.Wait()
			if len(published) != 2 {
				t.Fatalf("publications = %d, want 2", len(published))
			}
			allIDs := make(map[string]string)
			for _, signature := range published {
				prefix := root.ID + ":"
				if !strings.HasPrefix(signature.ID, prefix) {
					t.Fatalf("runtime root reused template ID %q", signature.ID)
				}
				suffix := strings.SplitN(strings.TrimPrefix(signature.ID, prefix), ":", 2)[0]
				if len(suffix) != timedRunSuffixLength {
					t.Fatalf("run suffix = %q", suffix)
				}
				walkTimedSignatureIDs(t, allIDs, signature, suffix, kind, make(map[*task.Signature]struct{}))
				if signature.Args[0].Value != "runtime argument" {
					t.Fatal("runtime pre-publication mutation did not execute")
				}
				if marker, _ := signature.Meta.Get("marker"); marker != "runtime" {
					t.Fatal("runtime metadata was not updated")
				}
			}
			if len(allIDs) != 2*wantNodes {
				t.Fatalf("reachable runtime IDs = %d, want %d", len(allIDs), 2*wantNodes)
			}
			after, err := json.Marshal(templates)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(before, after) {
				t.Fatal("cron fires mutated the registered task/callback templates")
			}
		})
	}
}

func TestTimedRunsIsolateNestedMetadata(t *testing.T) {
	for _, kind := range []string{"task", "chain"} {
		t.Run(kind, func(t *testing.T) {
			template := task.NewSignature("template", "task")
			template.Meta.Set("nested", map[string]interface{}{"counter": 0, "items": []int{10}})
			var published [][2]int
			server := &Server{
				backend: &groupTestBackend{},
				controller: &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
					value, _ := signature.Meta.Get("nested")
					nested := value.(map[string]interface{})
					published = append(published, [2]int{nested["counter"].(int), nested["items"].([]int)[0]})
					return nil
				}},
				lock: timedGroupTestLocker{}, scheduler: cron.New(),
				prePublishHandler: func(signature *task.Signature) {
					value, _ := signature.Meta.Get("nested")
					nested := value.(map[string]interface{})
					nested["counter"] = nested["counter"].(int) + 1
					nested["items"].([]int)[0]++
				},
			}
			var err error
			if kind == "task" {
				err = server.RegisteredTimedTask("* * * * *", "job", template)
			} else {
				err = server.RegisteredTimedChain("* * * * *", "job", template)
			}
			if err != nil {
				t.Fatal(err)
			}
			entries := server.scheduler.Entries()
			if len(entries) != 1 {
				t.Fatalf("jobs = %d, want 1", len(entries))
			}
			entries[0].Job.Run()
			entries[0].Job.Run()
			if len(published) != 2 || published[0] != [2]int{1, 11} || published[1] != [2]int{1, 11} {
				t.Errorf("published metadata = %v, want independent [1 11] values", published)
			}
			value, _ := template.Meta.Get("nested")
			nested := value.(map[string]interface{})
			if nested["counter"] != 0 || nested["items"].([]int)[0] != 10 {
				t.Errorf("registered template metadata changed: %v", nested)
			}
		})
	}
}
