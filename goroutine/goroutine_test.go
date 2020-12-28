/******
** @创建时间 : 2020/12/28 16:39
** @作者 : SongZhiBin
******/
package goroutine

import (
	"context"
	"fmt"
	"math"
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
			if math.Round(10) > 5 {
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
		time.Sleep(time.Second)
	}

	g.Shutdown()
}
