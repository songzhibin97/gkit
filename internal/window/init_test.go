/******
** @创建时间 : 2021/1/12 11:56
** @作者 : SongZhiBin
******/
package window

import (
	"strconv"
	"testing"
	"time"
)

func TestWindow(t *testing.T) {
	w := InitWindow()
	slice := []Index{
		{Name: "1", Score: 1}, {Name: "2", Score: 2},
		{Name: "2", Score: 2}, {Name: "3", Score: 3},
		{Name: "2", Score: 2}, {Name: "3", Score: 3},
		{Name: "4", Score: 4}, {Name: "3", Score: 3},
		{Name: "5", Score: 5}, {Name: "2", Score: 2},
		{Name: "6", Score: 6}, {Name: "5", Score: 5},
	}
	for i := 0; i < len(slice); i += 2 {
		w.AddIndex(slice[i].Name, slice[i].Score)
		w.AddIndex(slice[i+1].Name, slice[i+1].Score)
		time.Sleep(time.Second)
		t.Log(w.Show())
	}
}

func BenchmarkWindow(b *testing.B) {
	w := InitWindow()
	go func() {
		for {
			w.Show()
		}
	}()
	for i := 0; i < b.N; i++ {
		w.AddIndex(strconv.Itoa(i), uint(i))
	}

}
