/******
** @创建时间 : 2021/1/12 11:56
** @作者 : SongZhiBin
******/
package window

import (
	"strconv"
	"testing"
)

func TestWindow(t *testing.T) {
	w := InitWindow()
	for i := (uint)(1); i < 10000000; i++ {
		w.AddIndex(strconv.Itoa(int(i)),i)
		t.Log(w.Show())
	}
}
