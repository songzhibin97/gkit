/******
** @创建时间 : 2020/12/28 14:17
** @作者 : SongZhiBin
******/
package log

import "testing"

type testLogger struct {
	*testing.T
}

func (t *testLogger) Print(kv ...interface{}) {
	t.Log(kv...)
}

func Test_log_Print(t *testing.T) {
	log := &testLogger{t}
	log.Print("key","value")
	Debug(log).Print("key","value")
	Info(log).Print("key","value")
	Warn(log).Print("key","value")
	Error(log).Print("key","value")
}
