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

// loggerRef is a reference-counted handle to the active log file. The
// "current" slot owns one reference; each in-flight writeString that
// snapshots the handle owns one more. The underlying file is closed only
// when the last reference is dropped, so a rotation that retires the handle
// can never close a file out from under a concurrent writer (os.Stdout is
// never closed). This replaces the previous broken
// atomic.CompareAndSwapPointer swap, which raced on the pointer read and
// could close a file mid-write.
type loggerRef struct {
	file *os.File
	refs int32 // atomic; starts at 1 for the current-slot reference
}

func newLoggerRef(f *os.File) *loggerRef {
	return &loggerRef{file: f, refs: 1}
}

func (r *loggerRef) acquire() { atomic.AddInt32(&r.refs, 1) }

func (r *loggerRef) release() {
	if atomic.AddInt32(&r.refs, -1) == 0 && r.file != nil && r.file != os.Stdout {
		_ = r.file.Close()
	}
}

// acquireLogger returns the active logger handle with its reference count
// bumped; the caller MUST call release() on it when done. Reading the
// current handle and incrementing its count happen under the same lock
// setLogger uses to swap it, so a writer can never acquire a handle that
// setLogger has already retired.
func (w *Watching) acquireLogger() *loggerRef {
	w.config.L.RLock()
	ref := w.config.activeLog
	ref.acquire()
	w.config.L.RUnlock()
	return ref
}

// setLogger installs newLogger as the active log file and retires the
// previous handle. The retired file is NOT closed here; release() closes it
// once the last in-flight writer is done — never while the config lock is
// held and never under blocking file I/O.
func (w *Watching) setLogger(newLogger *os.File) {
	newRef := newLoggerRef(newLogger)
	w.config.L.Lock()
	old := w.config.activeLog
	if old != nil && old.file == newLogger {
		w.config.L.Unlock()
		return
	}
	w.config.activeLog = newRef
	w.config.Logger = newLogger // keep the mirror in sync for direct readers
	w.config.L.Unlock()
	if old != nil {
		old.release() // drop the current-slot reference
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
	ref := w.acquireLogger()
	defer ref.release()
	logger := ref.file
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
		w.rotate(ref)
	}
}

// rotate renames the active log file aside and installs a fresh one. The
// caller must hold the changeLog gate. It first re-validates that ref is
// still the active handle: another writer may have rotated since ref was
// acquired, in which case that rotation already replaced the file. Rotating
// again off a retired handle's size would rotate the new (possibly nearly
// empty) active file and, with the second-granularity suffix, collide with a
// same-second backup. Only the holder of the still-current handle rotates.
func (w *Watching) rotate(ref *loggerRef) {
	w.config.L.RLock()
	stale := w.config.activeLog != ref
	w.config.L.RUnlock()
	if stale {
		return
	}

	dumpPath := w.config.DumpPath
	suffix := time.Now().Format("20060102150405")
	srcPath := filepath.Clean(filepath.Join(dumpPath, defaultLoggerName))
	dstPath := srcPath + "_" + suffix + ".back"

	if err := os.Rename(srcPath, dstPath); err != nil {
		w.disableRotate()
		//nolint
		fmt.Println("rename err:", err, "from now on, it will be disabled split log")
		return
	}

	newLogger, err := os.OpenFile(filepath.Clean(srcPath), defaultLoggerFlags, defaultLoggerPerm)
	if err != nil {
		w.disableRotate()
		//nolint
		fmt.Println("open new file err:", err, "from now on, it will be disabled split log")
		return
	}

	w.setLogger(newLogger)
}
