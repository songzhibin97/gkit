package distributed

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/songzhibin97/gkit/distributed/task"
)

type timedGroupCaptureBackend struct {
	groupTestBackend

	captureMu sync.Mutex
	groupIDs  []string
	taskIDs   [][]string
	arrived   chan struct{}
	release   chan struct{}
}

func (b *timedGroupCaptureBackend) GroupTakeOver(groupID, _ string, taskIDs ...string) error {
	b.captureMu.Lock()
	b.groupIDs = append(b.groupIDs, groupID)
	b.taskIDs = append(b.taskIDs, append([]string(nil), taskIDs...))
	b.captureMu.Unlock()

	b.arrived <- struct{}{}
	<-b.release
	return nil
}

func (b *timedGroupCaptureBackend) snapshot() ([]string, [][]string) {
	b.captureMu.Lock()
	defer b.captureMu.Unlock()
	groupIDs := append([]string(nil), b.groupIDs...)
	taskIDs := make([][]string, len(b.taskIDs))
	for index := range b.taskIDs {
		taskIDs[index] = append([]string(nil), b.taskIDs[index]...)
	}
	return groupIDs, taskIDs
}

type timedGroupTestLocker struct{}

func (timedGroupTestLocker) Lock(string, int, string) error  { return nil }
func (timedGroupTestLocker) UnLock(string, string) error     { return nil }
func (timedGroupTestLocker) Renew(string, int, string) error { return nil }

type timedPublishedCapture struct {
	mu       sync.Mutex
	byGroup  map[string][]*task.Signature
	delegate *groupTestController
}

func newTimedPublishedCapture() *timedPublishedCapture {
	capture := &timedPublishedCapture{byGroup: make(map[string][]*task.Signature)}
	capture.delegate = &groupTestController{publishFn: func(_ context.Context, signature *task.Signature) error {
		capture.mu.Lock()
		defer capture.mu.Unlock()
		capture.byGroup[signature.GroupID] = append(capture.byGroup[signature.GroupID], signature)
		return nil
	}}
	return capture
}

func (c *timedPublishedCapture) snapshot() map[string][]*task.Signature {
	c.mu.Lock()
	defer c.mu.Unlock()
	copyByGroup := make(map[string][]*task.Signature, len(c.byGroup))
	for groupID, signatures := range c.byGroup {
		copyByGroup[groupID] = append([]*task.Signature(nil), signatures...)
	}
	return copyByGroup
}

func newTimedGroupTestServer(backend *timedGroupCaptureBackend, capture *timedPublishedCapture) *Server {
	return &Server{
		config:     &Config{ConsumeQueue: "issue-79-queue"},
		controller: capture.delegate,
		backend:    backend,
		lock:       timedGroupTestLocker{},
		scheduler:  cron.New(),
	}
}

func runTimedJobTwiceOverlapping(t *testing.T, server *Server, backend *timedGroupCaptureBackend) {
	t.Helper()
	entries := server.scheduler.Entries()
	if len(entries) != 1 {
		t.Fatalf("cron entry count = %d, want 1", len(entries))
	}

	done := make(chan struct{}, 2)
	for range []int{0, 1} {
		go func() {
			entries[0].Job.Run()
			done <- struct{}{}
		}()
	}
	for range []int{0, 1} {
		select {
		case <-backend.arrived:
		case <-time.After(time.Second):
			t.Fatal("overlapping timed group invocation did not reach GroupTakeOver")
		}
	}
	close(backend.release)
	for range []int{0, 1} {
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timed group invocation did not finish")
		}
	}
}

func assertTimedRunIDs(t *testing.T, groupTemplate string, groupIDs []string, takeoverTaskIDs [][]string, published map[string][]*task.Signature) {
	t.Helper()
	if len(groupIDs) != 2 {
		t.Fatalf("GroupTakeOver calls = %d, want 2", len(groupIDs))
	}
	if groupIDs[0] == groupIDs[1] {
		t.Fatalf("overlapping timed invocations reused group ID %q", groupIDs[0])
	}

	allIDs := make(map[string]string)
	takeoverIDs := make(map[string]string)
	for runIndex, groupID := range groupIDs {
		prefix := groupTemplate + ":"
		if !strings.HasPrefix(groupID, prefix) {
			t.Fatalf("runtime group ID %q does not retain template prefix %q", groupID, groupTemplate)
		}
		runSuffix := strings.TrimPrefix(groupID, prefix)
		if runSuffix == "" {
			t.Fatalf("runtime group ID %q has empty run suffix", groupID)
		}
		for _, id := range takeoverTaskIDs[runIndex] {
			assertUniqueTimedID(t, takeoverIDs, id, runSuffix, fmt.Sprintf("group %s takeover", groupID))
		}

		visited := make(map[*task.Signature]struct{})
		for _, signature := range published[groupID] {
			walkTimedSignatureIDs(t, allIDs, signature, runSuffix, groupID, visited)
		}
		for _, id := range takeoverTaskIDs[runIndex] {
			if _, ok := allIDs[id]; !ok {
				t.Fatalf("taken-over task ID %q was not published for group %q", id, groupID)
			}
		}
	}
}

func walkTimedSignatureIDs(t *testing.T, allIDs map[string]string, signature *task.Signature, runSuffix, groupID string, visited map[*task.Signature]struct{}) {
	t.Helper()
	if signature == nil {
		return
	}
	if _, ok := visited[signature]; ok {
		return
	}
	visited[signature] = struct{}{}
	assertUniqueTimedID(t, allIDs, signature.ID, runSuffix, "published graph "+groupID)
	for _, callback := range signature.CallbackOnSuccess {
		walkTimedSignatureIDs(t, allIDs, callback, runSuffix, groupID, visited)
	}
	for _, callback := range signature.CallbackOnError {
		walkTimedSignatureIDs(t, allIDs, callback, runSuffix, groupID, visited)
	}
	walkTimedSignatureIDs(t, allIDs, signature.CallbackChord, runSuffix, groupID, visited)
}

func assertUniqueTimedID(t *testing.T, allIDs map[string]string, id, runSuffix, location string) {
	t.Helper()
	if !strings.Contains(id, ":"+runSuffix+":") {
		t.Fatalf("runtime task ID %q does not contain group run suffix %q", id, runSuffix)
	}
	if previous, exists := allIDs[id]; exists {
		t.Fatalf("runtime task ID %q reused by %s and %s", id, previous, location)
	}
	allIDs[id] = location
}

func TestRegisteredTimedGroupUsesUniqueRuntimeIdentity(t *testing.T) {
	release := make(chan struct{})
	backend := &timedGroupCaptureBackend{arrived: make(chan struct{}, 2), release: release}
	capture := newTimedPublishedCapture()
	server := newTimedGroupTestServer(backend, capture)

	templateA := task.NewSignature("task-template", "task-a")
	templateA.CallbackOnSuccess = []*task.Signature{task.NewSignature("success-template", "success")}
	templateA.CallbackOnError = []*task.Signature{task.NewSignature("error-template", "error")}
	templateA.CallbackChord = task.NewSignature("chord-template", "chord")
	templateB := task.NewSignature("task-template", "task-b")
	if err := server.RegisteredTimedGroup("* * * * *", "timed-group", "group-template", 2, templateA, templateB); err != nil {
		t.Fatalf("RegisteredTimedGroup returned error: %v", err)
	}

	runTimedJobTwiceOverlapping(t, server, backend)
	groupIDs, taskIDs := backend.snapshot()
	assertTimedRunIDs(t, "group-template", groupIDs, taskIDs, capture.snapshot())

	if templateA.ID != "task-template" || templateB.ID != "task-template" ||
		templateA.GroupID != "-" || templateB.GroupID != "-" ||
		templateA.CallbackOnSuccess[0].ID != "success-template" ||
		templateA.CallbackOnError[0].ID != "error-template" ||
		templateA.CallbackChord.ID != "chord-template" {
		t.Fatalf("timed invocation mutated registered task templates: %#v %#v", templateA, templateB)
	}
}

func TestRegisteredTimedGroupCallbackCopiesAndRekeysCallbackTemplate(t *testing.T) {
	release := make(chan struct{})
	backend := &timedGroupCaptureBackend{arrived: make(chan struct{}, 2), release: release}
	capture := newTimedPublishedCapture()
	server := newTimedGroupTestServer(backend, capture)

	template := task.NewSignature("task-template", "task")
	callback := task.NewSignature("callback-template", "callback")
	callback.CallbackOnSuccess = []*task.Signature{task.NewSignature("callback-success-template", "after-callback")}
	if err := server.RegisteredTimedGroupCallback("* * * * *", "timed-callback", "callback-group-template", 1, callback, template); err != nil {
		t.Fatalf("RegisteredTimedGroupCallback returned error: %v", err)
	}

	runTimedJobTwiceOverlapping(t, server, backend)
	groupIDs, taskIDs := backend.snapshot()
	assertTimedRunIDs(t, "callback-group-template", groupIDs, taskIDs, capture.snapshot())

	if callback.ID != "callback-template" || callback.CallbackOnSuccess[0].ID != "callback-success-template" || template.CallbackChord != nil {
		t.Fatalf("timed callback invocation mutated registered templates: callback=%#v task=%#v", callback, template)
	}
}
