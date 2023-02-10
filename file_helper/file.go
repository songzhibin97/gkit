package file_helper

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/songzhibin97/gkit/options"

	"github.com/fsnotify/fsnotify"
)

var (
	ErrAlreadyExists = errors.New("file already exists")
	ErrFileNotFound  = errors.New("file not found")
	ErrAlreadyFinish = errors.New("file already finish")
	ErrReScan        = errors.New("re scan")
	ErrTimeout       = errors.New("timeout")
)

type config struct {
	endIdentification       string // 文件结束标识
	lineBreakIdentification byte   // 行标识

	prefix string

	bufSize int

	timeout time.Duration
}

const (
	defaultEndIdentification = "END"
	defaultLineBreak         = '\n'
	defaultBufSize           = 10
	defaultTimeout           = time.Hour
	defaultPrefix            = "./"
)

func SetEndIdentification(endIdentification string) options.Option {
	return func(o interface{}) {
		c := o.(*config)
		c.endIdentification = endIdentification
	}
}

func SetLineBreakIdentification(lineBreakIdentification byte) options.Option {
	return func(o interface{}) {
		c := o.(*config)
		c.lineBreakIdentification = lineBreakIdentification
	}
}

func SetPrefix(prefix string) options.Option {
	return func(o interface{}) {
		c := o.(*config)
		c.prefix = prefix
	}
}

func SetBufSize(bufSize int) options.Option {
	return func(o interface{}) {
		c := o.(*config)
		c.bufSize = bufSize
	}
}

func SetTimeout(timeout time.Duration) options.Option {
	return func(o interface{}) {
		c := o.(*config)
		c.timeout = timeout
	}
}

type fileBase struct {
	file     *os.File
	filePath string
	finish   int32 // 标记是否已经关闭句柄

	ctx context.Context

	config
}

func (f *fileBase) Close() {
	if f.file != nil {
		_ = f.file.Close()
	}
}

func (f *fileBase) FilePath() string {
	return f.filePath
}

func processFilePath(filename string, prefix string) string {
	if !filepath.IsAbs(filename) {
		filename = filepath.Join(prefix, filename)
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

	data = append(data, f.lineBreakIdentification)
	_, err := f.file.Write(data)
	return err
}

func (f *FileWriter) Finish() {
	if atomic.LoadInt32(&f.finish) == 1 {
		return
	}
	defer atomic.StoreInt32(&f.finish, 1)

	_ = f.Write([]byte(f.endIdentification))
	_ = f.file.Sync()
	f.Close()
}

func NewFileWrite(ctx context.Context, filename string, options ...options.Option) (*FileWriter, error) {
	c := config{
		endIdentification:       defaultEndIdentification,
		lineBreakIdentification: defaultLineBreak,
		prefix:                  defaultPrefix,
		bufSize:                 defaultBufSize,
		timeout:                 defaultTimeout,
	}
	for _, o := range options {
		o(&c)
	}

	filename = processFilePath(filename, c.prefix)
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
			ctx:      ctx,
			file:     w,
			filePath: filename,
			finish:   0,
			config:   c,
		},
	}, nil
}

type FileReader struct {
	*fileBase

	readerBuff   *bufio.Reader // 只读模式
	lastScanTime int64

	buffer chan []byte

	info string
}

func NewFileReader(ctx context.Context, filename string, options ...options.Option) (*FileReader, error) {

	c := config{
		endIdentification:       defaultEndIdentification,
		lineBreakIdentification: defaultLineBreak,
		prefix:                  defaultPrefix,
		bufSize:                 defaultBufSize,
		timeout:                 defaultTimeout,
	}

	for _, o := range options {
		o(&c)
	}

	filename = processFilePath(filename, c.prefix)
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
			file:     r,
			filePath: filename,
			ctx:      ctx,
		},
		readerBuff:   scan,
		buffer:       make(chan []byte, c.bufSize),
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
		timer := time.NewTicker(f.timeout/2 + 1)
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
							f.info = fmt.Sprintf("file %s has been deleted", filename)
							return
						default:

						}
					case <-timer.C:

					case <-f.ctx.Done():
						// ctx 终止
						f.info = fmt.Sprintf("file %s has been canceled", filename)
						return

					}
				default:
					// 其他错误
					f.info = fmt.Sprintf("file %s has been error %s", filename, err.Error())
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
	bs, err := f.readerBuff.ReadBytes(f.lineBreakIdentification)
	if err != nil && errors.Is(err, io.EOF) {
		if f.timeout != 0 && time.Now().Unix()-f.lastScanTime > int64(f.timeout/time.Second) {
			return nil, ErrTimeout
		}
		return nil, ErrReScan
	}

	bs = bytes.TrimSuffix(bs, []byte{f.lineBreakIdentification})
	if len(bs) == len(f.endIdentification) && string(bs) == f.endIdentification {
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
