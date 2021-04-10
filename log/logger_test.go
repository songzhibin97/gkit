package log

import "testing"

type testLogger struct {
	*testing.T
}

func (t *testLogger) Print(kv ...interface{}) {
	t.Log(kv...)
}

func Test_log_Print(t *testing.T) {
	logs := &testLogger{t}
	logs.Print("key", "value")
	Debug(logs).Print("key", "value")
	Info(logs).Print("key", "value")
	Warn(logs).Print("key", "value")
	Error(logs).Print("key", "value")
}
