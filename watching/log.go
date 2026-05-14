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

// currentLogger returns the active logger under the config RWMutex. The
// previous code read `w.config.Logger` non-atomically while another
// goroutine performed an `atomic.CompareAndSwapPointer` on the same field
// whose expected-old-value was ALSO read non-atomically — the CAS was
// theatre, the read was a data race.
func (w *Watching) currentLogger() *os.File {
	w.config.L.RLock()
	defer w.config.L.RUnlock()
	return w.config.Logger
}

// setLogger swaps the active logger under the config write lock, closing
// the previous one if it differs and isn't Stdout.
func (w *Watching) setLogger(newLogger *os.File) {
	w.config.L.Lock()
	old := w.config.Logger
	w.config.Logger = newLogger
	w.config.L.Unlock()
	if old != nil && old != newLogger && old != os.Stdout {
		_ = old.Close()
	}
}

func (w *Watching) rotateEnabled() bool {
	w.config.L.RLock()
	defer w.config.L.RUnlock()
	return w.config.logConfigs.RotateEnable
}

func (w *Watching) disableRotate() {
	w.config.L.Lock()
	w.config.logConfigs.RotateEnable = false
	w.config.L.Unlock()
}

func (w *Watching) writeString(content string) {
	logger := w.currentLogger()
	if _, err := logger.WriteString(content); err != nil {
		fmt.Println(err) // where to write this log?
	}

	if !w.rotateEnabled() {
		return
	}

	state, err := logger.Stat()
	if err != nil {
		w.disableRotate()
		//nolint
		fmt.Println("get logger stat:", err, "from now on, it will be disabled split log")

		return
	}

	if state.Size() > w.config.logConfigs.SplitLoggerSize && atomic.CompareAndSwapInt32(&w.changeLog, 0, 1) {
		defer atomic.StoreInt32(&w.changeLog, 0)

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
			w.disableRotate()
			//nolint
			fmt.Println("rename err:", err, "from now on, it will be disabled split log")

			return
		}

		newLogger, err = os.OpenFile(filepath.Clean(srcPath), defaultLoggerFlags, defaultLoggerPerm)

		if err != nil {
			w.disableRotate()
			//nolint
			fmt.Println("open new file err:", err, "from now on, it will be disabled split log")

			return
		}

		w.setLogger(newLogger)
	}
}
