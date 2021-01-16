package cpu

import (
	"reflect"
	"testing"
	"time"
)

func Test_CPUUsage(t *testing.T) {
	var stat Stat
	ReadStat(&stat)
	t.Log(stat)
	time.Sleep(time.Millisecond * 1000)
	for i := 0; i < 6; i++ {
		time.Sleep(time.Millisecond * 500)
		ReadStat(&stat)
		if stat.Usage == 0 {
			t.Fatalf("get cpu failed!cpu usage is zero!")
		}
		t.Log(stat)
	}
}

func TestStat(t *testing.T) {
	time.Sleep(time.Second * 2)
	var s Stat
	var i Info
	ReadStat(&s)
	i = GetInfo()

	t.Log(reflect.DeepEqual(s.Usage, 0))
	t.Log(reflect.DeepEqual(i.Frequency,0))
	t.Log(reflect.DeepEqual(i.Quota,0))
}
