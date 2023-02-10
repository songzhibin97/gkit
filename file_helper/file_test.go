package file_helper

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"
)

func TestNewFileWrite(t *testing.T) {

	w, err := NewFileWrite(context.Background(), "test.txt")
	if err != nil {
		t.Error(err)
		return
	}
	defer w.Finish()
	for i := 0; i < 10; i++ {
		err = w.Write([]byte("hello world" + strconv.Itoa(i)))
		if err != nil {
			t.Error(err)
		}
	}
}

func TestAdd(t *testing.T) {
	w, err := os.OpenFile("test.txt", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		t.Error(err)
	}
	for i := 0; i < 3; i++ {
		w.Write([]byte("hello world" + strconv.Itoa(i) + "\n"))
	}
}

func TestNewFileReader(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	r, err := NewFileReader(ctx, "test.txt", SetTimeout(time.Minute))
	if err != nil {
		t.Error(err)
		return
	}
	defer r.Close()
	defer func() {
		t.Log(r.Info())
	}()
	for {
		v, err := r.ReadLine()
		if err != nil {
			t.Error(err)
			return
		}
		t.Log(string(v))
	}
}
