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
		g.Trick()
		if i == 7 {
			g.ChangeMax(5)
		}
	}
	_ = g.Shutdown()
}

func TestSetIdle(t *testing.T) {
	g := NewGoroutine(context.Background(), SetMax(1000), SetIdle(10), SetCheckTime(time.Second), SetLogger(log.DefaultLogger))
	for i := 0; i < 10000; i++ {
		g.AddTask(func() {
			func(i int) {
				time.Sleep(time.Second)
				// t.Log("close", i)
			}(i)
		})
	}
	for i := 0; i < 20; i++ {
		g.Trick()
		time.Sleep(time.Second)
	}
}
