package goroutine

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/log"
)

func TestNewGoroutine(t *testing.T) {
	g := NewGoroutine(context.Background(), SetMax(10), SetLogger(log.DefaultLogger))
	for i := 0; i < 20; i++ {
		i := i
		fmt.Println(g.AddTask(func() {
			//if rand.Int31n(10) > 5 {
			//	panic(i)
			//}
			fmt.Println("start:", i)
			time.Sleep(5 * time.Second)
			fmt.Println("end:", i)
		}))
		g.trick()
		if i == 7 {
			g.ChangeMax(5)
		}
	}
	_ = g.Shutdown()
}
