package file_helper

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

const DefaultDir = "./"

var (
	ErrAlreadyExists = errors.New("file already exists")
	ErrFileNotFound  = errors.New("file not found")
	ErrAlreadyFinish = errors.New("file already finish")
	ErrReScan        = errors.New("re scan")
	ErrTimeout       = errors.New("timeout")
)

type fileBase struct {
	file     *os.File
	filePath string
	finish   int32 // 标记是否已经关闭句柄

	EndIdentification string // 文件结束标识

}

func (f *fileBase) Close() {
	if f.file != nil {
		_ = f.file.Close()
	}
}

func (f *fileBase) FilePath() string {
	return f.filePath
}

func processFilePath(filename string, prefix ...string) string {
	if !filepath.IsAbs(filename) {
		prefix = append(prefix, DefaultDir)
		filename = filepath.Join(prefix[0], filename)
	}
	return filename
}

type FileWriter struct {
	*fileBase // 只追加写
}

func (f *FileWriter) Write(data []byte) error {
	if atomic.LoadInt32(&f.finish) == 1 {
		return ErrAlreadyFinish
	}

	data = append(data, '\n')
	_, err := f.file.Write(data)
	return err
}

func (f *FileWriter) Finish() {
	if atomic.LoadInt32(&f.finish) == 1 {
		return
	}
	defer atomic.StoreInt32(&f.finish, 1)

	_ = f.Write([]byte(f.EndIdentification))
	_ = f.file.Sync()
	f.Close()
}

func NewFileWrite(filename string, endIdentification string, prefix ...string) (*FileWriter, error) {
	filename = processFilePath(filename, prefix...)
	_, err := os.Stat(filename)
	if err == nil {
		return nil, ErrAlreadyExists
	}
	_, err = os.Stat(path.Dir(filename))
	if err != nil {
		err = os.Mkdir(path.Dir(filename), 0666)
		if err != nil {
			return nil, err
		}
	}
	w, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return &FileWriter{
		fileBase: &fileBase{
			file:              w,
			filePath:          filename,
			finish:            0,
			EndIdentification: endIdentification,
		},
	}, nil
}

type FileReader struct {
	*fileBase

	readerBuff   *bufio.Reader // 只读模式
	lastScanTime int64

	timeOut time.Duration

	buffer chan []byte
}

func NewFileReader(filename string, endIdentification string, buf int, timeout time.Duration, prefix ...string) (*FileReader, error) {
	filename = processFilePath(filename, prefix...)
	_, err := os.Stat(filename)
	if err != nil {
		return nil, ErrFileNotFound
	}
	r, err := os.OpenFile(filename, os.O_RDONLY, 0666)
	if err != nil {
		return nil, err
	}
	scan := bufio.NewReader(r)

	f := &FileReader{
		fileBase: &fileBase{
			file:              r,
			filePath:          filename,
			EndIdentification: endIdentification,
		},
		readerBuff:   scan,
		buffer:       make(chan []byte, buf),
		timeOut:      timeout,
		lastScanTime: time.Now().Unix(),
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	err = watcher.Add(filename)
	if err != nil {
		return nil, err
	}

	go func() {
		timer := time.NewTicker(f.timeOut/2 + 1)
		defer close(f.buffer)
		defer func() { timer.Stop() }()
		for {
			line, err := f.readLine()
			if err != nil {
				switch err {
				case ErrReScan:
					select {
					case env := <-watcher.Events:
						switch env.Op {
						case fsnotify.Remove:
							// 文件被删除 退出
							return
						default:

						}
					case <-timer.C:
					}
				default:
					return
				}
			} else {
				f.buffer <- line
			}
		}
	}()
	return f, nil
}

func (f *FileReader) readLine() ([]byte, error) {

	if atomic.LoadInt32(&f.finish) == 1 {
		f.Close()
		return nil, io.EOF
	}
	bs, err := f.readerBuff.ReadBytes('\n')
	if err != nil && errors.Is(err, io.EOF) {
		if f.timeOut != 0 && time.Now().Unix()-f.lastScanTime > int64(f.timeOut/time.Second) {
			return nil, ErrTimeout
		}
		return nil, ErrReScan
	}

	bs = bytes.TrimSuffix(bs, []byte{'\n'})
	if len(bs) == len(f.EndIdentification) && string(bs) == f.EndIdentification {
		if atomic.CompareAndSwapInt32(&f.finish, 0, 1) {
			f.Close()
		}
		return nil, io.EOF
	}

	atomic.StoreInt64(&f.lastScanTime, time.Now().Unix())
	return bs, nil
}

func (f *FileReader) ReadLine() ([]byte, error) {

	v, ok := <-f.buffer
	if !ok {
		return nil, io.EOF
	}

	return v, nil
}

func (f *FileReader) ReadLineSync() ([]byte, error) {
	select {
	case v, ok := <-f.buffer:
		if !ok {
			return nil, io.EOF
		}
		return v, nil
	default:
		return nil, ErrReScan
	}
}
