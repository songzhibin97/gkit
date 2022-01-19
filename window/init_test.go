package window

import (
	"strconv"
	"testing"
	"time"
)

func TestWindow(t *testing.T) {
	w := NewWindow()
	slice := []Index{
		{Name: "1", Score: 1},
		{Name: "2", Score: 2},
		{Name: "2", Score: 2},
		{Name: "3", Score: 3},
		{Name: "2", Score: 2},
		{Name: "3", Score: 3},
		{Name: "4", Score: 4},
		{Name: "3", Score: 3},
		{Name: "5", Score: 5},
		{Name: "2", Score: 2},
		{Name: "6", Score: 6},
		{Name: "5", Score: 5},
	}
	/*
			[{1 1} {2 2}]
		    [{2 4} {3 3} {1 1}]
		    [{1 1} {2 6} {3 6}]
		    [{3 9} {4 4} {1 1} {2 6}]
		    [{1 1} {2 8} {3 9} {4 4} {5 5}]
		    [{5 10} {3 9} {2 6} {4 4} {6 6}]
	*/
	for i := 0; i < len(slice); i += 2 {
		w.AddIndex(slice[i].Name, slice[i].Score)
		w.AddIndex(slice[i+1].Name, slice[i+1].Score)
		time.Sleep(time.Second)
		t.Log(w.Show())
	}
}

func BenchmarkWindow(b *testing.B) {
	w := NewWindow()
	for i := 0; i < b.N; i++ {
		w.AddIndex(strconv.Itoa(i), uint(i))
		w.Show()
	}
}
