package watching

import (
	"fmt"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

// log write content to log file.
func (w *Watching) logf(pattern string, args ...interface{}) {
	if w.config.LogLevel >= LogLevelInfo {
		timestamp := "[" + time.Now().Format("2006-01-02 15:04:05.000") + "]"
		w.writeString(fmt.Sprintf(timestamp+pattern+"\n", args...))
	}
}

// log write content to log file.
func (w *Watching) debugf(pattern string, args ...interface{}) {
	if w.config.LogLevel >= LogLevelDebug {
		w.writeString(fmt.Sprintf(pattern+"\n", args...))
	}
}

func (w *Watching) writeString(content string) {
	if _, err := w.config.Logger.WriteString(content); err != nil {
		fmt.Println(err) // where to write this log?
	}

	if !w.config.logConfigs.RotateEnable {
		return
	}

	state, err := w.config.Logger.Stat()
	if err != nil {
		w.config.logConfigs.RotateEnable = false
		//nolint
		fmt.Println("get logger stat:", err, "from now on, it will be disabled split log")

		return
	}

	if state.Size() > w.config.logConfigs.SplitLoggerSize && atomic.CompareAndSwapInt32(&w.config.logConfigs.Changelog, 0, 1) {
		defer atomic.StoreInt32(&w.config.logConfigs.Changelog, 0)

		var (
			newLogger *os.File
			err       error
			dumpPath  = w.config.DumpPath
			suffix    = time.Now().Format("20060102150405")
			srcPath   = filepath.Clean(filepath.Join(dumpPath, defaultLoggerName))
			dstPath   = srcPath + "_" + suffix + ".back"
		)

		err = os.Rename(srcPath, dstPath)

		if err != nil {
			w.config.logConfigs.RotateEnable = false
			//nolint
			fmt.Println("rename err:", err, "from now on, it will be disabled split log")

			return
		}

		newLogger, err = os.OpenFile(filepath.Clean(srcPath), defaultLoggerFlags, defaultLoggerPerm)

		if err != nil {
			w.config.logConfigs.RotateEnable = false
			//nolint
			fmt.Println("open new file err:", err, "from now on, it will be disabled split log")

			return
		}

		w.config.Logger, newLogger = newLogger, w.config.Logger
		_ = newLogger.Close()
	}
}
