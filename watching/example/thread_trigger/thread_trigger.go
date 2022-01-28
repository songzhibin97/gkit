package main

/*
#include <stdio.h>
#include <stdlib.h>
#include <unistd.h>
void output(char *str) {
    sleep(10000);
    printf("%s\n", str);
}
*/
import "C"

import (
	"fmt"
	"net/http"
	"time"
	"unsafe"

	"github.com/songzhibin97/gkit/watching"

	_ "net/http/pprof"
)

func init() {
	go func() {
		w := watching.NewWatching(
			watching.WithCollectInterval("2s"),
			watching.WithCoolDown("5s"),
			watching.WithDumpPath("/tmp"),
			watching.WithTextDump(),
			watching.WithThreadDump(10, 25, 100),
		)
		w.EnableThreadDump().Start()
		time.Sleep(time.Hour)
	}()
}

func leak(wr http.ResponseWriter, req *http.Request) {
	go func() {
		for i := 0; i < 1000; i++ {
			go func() {
				str := "hello cgo"
				// change to char*
				cstr := C.CString(str)
				C.output(cstr)
				C.free(unsafe.Pointer(cstr))
			}()
		}
	}()
}

func main() {
	http.HandleFunc("/leak", leak)
	err := http.ListenAndServe(":10003", nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	select {}
}
