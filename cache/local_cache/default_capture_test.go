package local_cache

import (
	"context"
	"os"
	"os/exec"
	"reflect"
	"testing"
	"time"
)

// Regression for #164, item 01-06. Run in a child process so stdout capture
// cannot race with other tests or their janitor goroutines. All data is a
// synthetic fixture; the test never reads application cache contents.
func TestCacheDefaultCaptureSilent(t *testing.T) {
	const helperEnv = "GKIT_TEST_DEFAULT_CAPTURE_OPERATION"
	if operation := os.Getenv(helperEnv); operation != "" {
		checkDefaultCaptureDeletion(t, operation)
		if t.Failed() {
			os.Exit(1)
		}
		os.Exit(0)
	}
	for _, operation := range []string{"Delete", "DeleteExpire"} {
		t.Run(operation, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestCacheDefaultCaptureSilent$")
			cmd.Env = append(os.Environ(), helperEnv+"="+operation)
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("deletion subprocess failed: %v; output: %s", err, output)
			}
			if len(output) != 0 {
				t.Fatalf("default %s wrote to stdout/stderr: %q", operation, output)
			}
		})
	}
}

func checkDefaultCaptureDeletion(t *testing.T, operation string) {
	t.Helper()
	const key, value = "synthetic-cache-key", "synthetic-cache-value"
	item := Iterator{Val: value}
	if operation == "DeleteExpire" {
		item.Expire = 1 // Already expired, with no scheduling or sleep required.
	}
	c := NewCache(SetMember(map[string]Iterator{key: item}))
	defer func() {
		if err := c.Shutdown(); err != nil {
			t.Error(err)
		}
	}()
	if got := c.Count(); got != 1 {
		t.Fatalf("initial Count = %d, want 1", got)
	}
	switch operation {
	case "Delete":
		c.Delete(key)
	case "DeleteExpire":
		c.DeleteExpire()
	default:
		t.Fatalf("unknown deletion operation %q", operation)
	}
	// Check Count before Get, since Get could itself evict an expired entry.
	if got := c.Count(); got != 0 {
		t.Fatalf("Count after %s = %d, want 0", operation, got)
	}
	if got, found := c.Get(key); found || got != nil {
		t.Fatalf("deleted Get = (%v, %t), want (nil, false)", got, found)
	}
}

func TestCacheExplicitCaptureStillReceivesDeletedValues(t *testing.T) {
	for _, setup := range []string{"SetCapture", "ChangeCapture"} {
		t.Run(setup, func(t *testing.T) {
			var got []kv
			capture := func(key string, value interface{}) {
				got = append(got, kv{key: key, value: value})
			}
			member := map[string]Iterator{
				"deleted": {Val: "value"},
				"expired": {Val: 42, Expire: 1},
			}
			var c Cache
			if setup == "SetCapture" {
				c = NewCache(SetMember(member), SetCapture(capture))
			} else {
				c = NewCache(SetMember(member))
				c.ChangeCapture(capture)
			}
			defer func() {
				if err := c.Shutdown(); err != nil {
					t.Error(err)
				}
			}()
			c.Delete("deleted")
			c.DeleteExpire()
			want := []kv{{key: "deleted", value: "value"}, {key: "expired", value: 42}}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("capture events = %#v, want %#v", got, want)
			}
			if count := c.Count(); count != 0 {
				t.Fatalf("Count after explicit capture = %d, want 0", count)
			}
			c.ChangeCapture(nil)
			c.Set("disabled", "ignored", NoExpire)
			c.Delete("disabled")
			if !reflect.DeepEqual(got, want) || c.Count() != 0 {
				t.Fatalf("disabled capture produced events or prevented deletion: events=%#v, Count=%d", got, c.Count())
			}
		})
	}
}
