package backend_mongodb

import (
	"fmt"
	"sync"
	"testing"
)

func TestIssue167RuntimeRetentionRace(t *testing.T) {
	b := issue167Mongo(t)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			b.SetResultExpire(int64(60 + i%2))
		}
	}()
	defer wg.Wait()
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("issue167-retention-%d", i)
		if err := b.GroupTakeOver(id, "retention", "member"); err != nil {
			t.Fatal(err)
		}
		group, err := b.getGroup(id)
		if err != nil || (group.TTL != 60 && group.TTL != 61 && group.TTL != -1) {
			t.Fatalf("group TTL %v %v", group, err)
		}
	}
}
