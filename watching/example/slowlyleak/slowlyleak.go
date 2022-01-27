package main

import (
	"net/http"
	"time"

	"github.com/songzhibin97/gkit/watching"
)

func init() {
	http.HandleFunc("/leak", leak)
	go http.ListenAndServe(":10003", nil)
}

func main() {
	w := watching.NewWatching(
		watching.WithCollectInterval("2s"),
		watching.WithCoolDown("1m"),
		watching.WithDumpPath("/tmp"),
		watching.WithTextDump(),
		watching.WithGoroutineDump(10, 25, 80, 1000),
	)
	w.EnableGoroutineDump().Start()
	time.Sleep(time.Hour)
}

func leak(wr http.ResponseWriter, req *http.Request) {
	taskChan := make(chan int)
	consumer := func() {
		for task := range taskChan {
			_ = task // do some tasks
		}
	}

	producer := func() {
		for i := 0; i < 10; i++ {
			taskChan <- i // generate some tasks
		}
		// forget to close the taskChan here
	}

	go consumer()
	go producer()
}
