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
	for i := (uint)(0); i < 1000; i++ {
		w.AddIndex(strconv.Itoa(int(i)),i)
		time.Sleep(time.Second/2)
		t.Log(w.Show())
	}
}
