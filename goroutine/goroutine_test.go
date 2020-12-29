package goroutine

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

type testLogger struct {
	*testing.T
}

func (t *testLogger) Print(kv ...interface{}) {
	t.Log(kv...)
}
func TestNewGoroutine(t *testing.T) {
	g := NewGoroutine(context.Background(), 10, &testLogger{t})
	for i := 0; i < 100; i++ {
		fmt.Println(g.AddTask(func() {
			if rand.Int31n(10) > 5 {
				panic(i)
			}
			fmt.Println("start:", i)
			time.Sleep(5 * time.Second)
			fmt.Println("end:", i)
		}))
		g.trick()
		if i == 26 {
			g.ChangeMax(5)
		}
	}

	g.Shutdown()
}
