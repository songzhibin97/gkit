package log

import (
	"testing"
)

func TestDebugHelper(t *testing.T) {
	logs := NewHelper(&testLogger{t}, LevelDebug)
	logs.Debug("debug", "v")
	logs.Debugf("%s,%s", "debugf", "v")
	logs.Info("Info", "v")
	logs.Infof("%s,%s", "infof", "v")
	logs.Warn("Warn", "v")
	logs.Warnf("%s,%s", "warnf", "v")
	logs.Error("Error", "v")
	logs.Errorf("%s,%s", "errorf", "v")
}

func TestInfoHelper(t *testing.T) {
	log := NewHelper(&testLogger{t}, LevelInfo)
	log.Debug("debug", "v")
	log.Debugf("%s,%s\n", "debugf", "v")
	log.Info("Info", "v")
	log.Infof("%s,%s\n", "infof", "v")
	log.Warn("Warn", "v")
	log.Warnf("%s,%s\n", "warnf", "v")
	log.Error("Error", "v")
	log.Errorf("%s,%s\n", "errorf", "v")
}
func TestWarnHelper(t *testing.T) {
	log := NewHelper(&testLogger{t}, LevelWarn)
	log.Debug("debug", "v")
	log.Debugf("%s,%s\n", "debugf", "v")
	log.Info("Info", "v")
	log.Infof("%s,%s\n", "infof", "v")
	log.Warn("Warn", "v")
	log.Warnf("%s,%s\n", "warnf", "v")
	log.Error("Error", "v")
	log.Errorf("%s,%s\n", "errorf", "v")
}
func TestErrorHelper(t *testing.T) {
	log := NewHelper(&testLogger{t}, LevelError)
	log.Debug("debug", "v")
	log.Debugf("%s,%s\n", "debugf", "v")
	log.Info("Info", "v")
	log.Infof("%s,%s\n", "infof", "v")
	log.Warn("Warn", "v")
	log.Warnf("%s,%s\n", "warnf", "v")
	log.Error("Error", "v")
	log.Errorf("%s,%s\n", "errorf", "v")
}
