package log

import (
	"testing"
)

func TestDebugHelper(t *testing.T) {
	log := NewHelper(&testLogger{t}, LevelDebug)
	log.Debug("debug", "v")
	log.Debugf("%s,%s", "debugf", "v")
	log.Info("Info", "v")
	log.Infof("%s,%s", "infof", "v")
	log.Warn("Warn", "v")
	log.Warnf("%s,%s", "warnf", "v")
	log.Error("Error", "v")
	log.Errorf("%s,%s", "errorf", "v")
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
