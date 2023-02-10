package file_helper

import (
	"os"
	"strconv"
	"testing"
	"time"
)

func TestNewFileWrite(t *testing.T) {
	w, err := NewFileWrite("test.txt", "END")
	if err != nil {
		t.Error(err)
	}
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
	r, err := NewFileReader("test.txt", "END", 0, time.Minute)
	if err != nil {
		t.Error(err)
	}
	for {
		v, err := r.ReadLine()
		if err != nil {
			t.Error(err)
			return
		}
		t.Log(string(v))
	}
}
