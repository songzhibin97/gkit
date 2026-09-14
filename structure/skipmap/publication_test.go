package skipmap

import (
	"sync/atomic"
	"testing"
	"time"
)

// Hold a real insertion immediately before fullyLinked is published. No random
// height or competing insertion is needed to exercise the existing-node paths.
func TestExistingWriteWaitsForPublication(t *testing.T) {
	for _, kind := range []string{"Int64", "IntDesc", "String"} {
		for _, operation := range []string{"Store", "LoadOrStore", "LoadOrStoreLazy"} {
			t.Run(kind+"/"+operation, func(t *testing.T) {
				var flags *bitflag
				var write func() (interface{}, bool)
				var load func() (interface{}, bool)
				var deleteNode func() bool
				var finish func()
				var calls int32
				lazy := func() interface{} { atomic.AddInt32(&calls, 1); return "replacement" }
				switch kind {
				case "Int64":
					m := NewInt64()
					n := newInt64Node(7, "initial", 1)
					m.header.mu.Lock()
					m.header.atomicStoreNext(0, n)
					flags = &n.flags
					load = func() (interface{}, bool) { return m.Load(7) }
					deleteNode = func() bool { return m.Delete(7) }
					finish = func() { n.flags.SetTrue(fullyLinked); m.header.mu.Unlock(); atomic.AddInt64(&m.length, 1) }
					write = func() (interface{}, bool) {
						switch operation {
						case "Store":
							m.Store(7, "replacement")
							return "replacement", false
						case "LoadOrStore":
							return m.LoadOrStore(7, "replacement")
						default:
							return m.LoadOrStoreLazy(7, lazy)
						}
					}
				case "IntDesc":
					m := NewIntDesc()
					n := newIntNodeDesc(7, "initial", 1)
					m.header.mu.Lock()
					m.header.atomicStoreNext(0, n)
					flags = &n.flags
					load = func() (interface{}, bool) { return m.Load(7) }
					deleteNode = func() bool { return m.Delete(7) }
					finish = func() { n.flags.SetTrue(fullyLinked); m.header.mu.Unlock(); atomic.AddInt64(&m.length, 1) }
					write = func() (interface{}, bool) {
						switch operation {
						case "Store":
							m.Store(7, "replacement")
							return "replacement", false
						case "LoadOrStore":
							return m.LoadOrStore(7, "replacement")
						default:
							return m.LoadOrStoreLazy(7, lazy)
						}
					}
				case "String":
					m := NewString()
					n := newStringNode("key", "initial", 1)
					m.header.mu.Lock()
					m.header.atomicStoreNext(0, n)
					flags = &n.flags
					load = func() (interface{}, bool) { return m.Load("key") }
					deleteNode = func() bool { return m.Delete("key") }
					finish = func() { n.flags.SetTrue(fullyLinked); m.header.mu.Unlock(); atomic.AddInt64(&m.length, 1) }
					write = func() (interface{}, bool) {
						switch operation {
						case "Store":
							m.Store("key", "replacement")
							return "replacement", false
						case "LoadOrStore":
							return m.LoadOrStore("key", "replacement")
						default:
							return m.LoadOrStoreLazy("key", lazy)
						}
					}
				}
				if flags.Get(fullyLinked) {
					t.Fatal("fixture is already published")
				}
				if value, ok := load(); ok || value != nil {
					t.Fatalf("unpublished Load = %v, %t", value, ok)
				}
				if deleteNode() || flags.Get(marked) {
					t.Fatal("Delete marked an unpublished insertion")
				}
				type result struct {
					value  interface{}
					loaded bool
				}
				done := make(chan result, 1)
				started := make(chan struct{})
				go func() { close(started); value, loaded := write(); done <- result{value, loaded} }()
				<-started
				var got result
				returned := false
				select {
				case got = <-done:
					returned = true
					t.Errorf("%s returned before fullyLinked: %v, %t", operation, got.value, got.loaded)
				case <-time.After(25 * time.Millisecond):
					// A deadline only bounds the negative assertion; publication is controlled
					// by the fixture, never by a sleep or a randomly selected node height.
				}
				finish()
				if !returned {
					select {
					case got = <-done:
					case <-time.After(time.Second):
						t.Fatal("write did not finish after publication")
					}
				}
				want, wantLoaded := "initial", true
				if operation == "Store" {
					want, wantLoaded = "replacement", false
				}
				if got.value != want || got.loaded != wantLoaded {
					t.Errorf("write = %v, %t; want %s, %t", got.value, got.loaded, want, wantLoaded)
				}
				if value, ok := load(); !ok || value != want {
					t.Errorf("published Load = %v, %t; want %s, true", value, ok, want)
				}
				if got := atomic.LoadInt32(&calls); got != 0 {
					t.Errorf("existing-key lazy calls = %d, want 0", got)
				}
				if !deleteNode() {
					t.Fatal("published node could not be deleted")
				}
				if value, ok := load(); ok || value != nil {
					t.Fatalf("deleted Load = %v, %t", value, ok)
				}
			})
		}
	}
}

func TestWriteRetriesMarkedNode(t *testing.T) {
	for _, operation := range []string{"Store", "LoadOrStore", "LoadOrStoreLazy"} {
		t.Run(operation, func(t *testing.T) {
			m := NewInt64()
			m.Store(7, "old")
			n := m.header.atomicLoadNext(0)
			// Pause Delete after logical deletion, before unlinking. Writers must retry
			// their search, so they can insert a new node after this one is unlinked.
			n.mu.Lock()
			n.flags.SetTrue(marked)
			m.header.mu.Lock()
			done := make(chan struct{})
			started := make(chan struct{})
			var actual interface{}
			var loaded bool
			var calls int32
			go func() {
				close(started)
				switch operation {
				case "Store":
					m.Store(7, "new")
					actual = "new"
				case "LoadOrStore":
					actual, loaded = m.LoadOrStore(7, "new")
				default:
					actual, loaded = m.LoadOrStoreLazy(7, func() interface{} { atomic.AddInt32(&calls, 1); return "new" })
				}
				close(done)
			}()
			<-started
			select {
			case <-done:
				t.Error("write returned while the old node was marked and linked")
			case <-time.After(25 * time.Millisecond):
			}
			if got := atomic.LoadInt32(&calls); got != 0 {
				t.Errorf("lazy callback ran before insertion was possible: %d calls", got)
			}
			for i := int(n.level) - 1; i >= 0; i-- {
				m.header.atomicStoreNext(i, nil)
			}
			n.mu.Unlock()
			m.header.mu.Unlock()
			atomic.AddInt64(&m.length, -1)
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("write did not retry after physical deletion")
			}
			if actual != "new" || loaded {
				t.Errorf("write = %v, %t; want new, false", actual, loaded)
			}
			if value, ok := m.Load(7); !ok || value != "new" {
				t.Errorf("Load = %v, %t; want new, true", value, ok)
			}
			if m.Len() != 1 {
				t.Errorf("Len = %d, want 1", m.Len())
			}
			wantCalls := int32(0)
			if operation == "LoadOrStoreLazy" {
				wantCalls = 1
			}
			if got := atomic.LoadInt32(&calls); got != wantCalls {
				t.Errorf("callback calls = %d, want %d", got, wantCalls)
			}
		})
	}
}
