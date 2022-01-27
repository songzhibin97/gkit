package main

import (
	"net/http"
	"time"

	"github.com/songzhibin97/gkit/watching"
)

func init() {
	http.HandleFunc("/chanblock", channelBlock)
	go http.ListenAndServe(":10003", nil)
}

func main() {
	w := watching.NewWatching(
		watching.WithCollectInterval("5s"),
		watching.WithCoolDown("1m"),
		watching.WithDumpPath("/tmp"),
		watching.WithTextDump(),
		watching.WithGoroutineDump(10, 25, 2000, 74),
	)
	w.EnableGoroutineDump().Start()
	time.Sleep(time.Hour)
}

var nilCh chan int

func channelBlock(wr http.ResponseWriter, req *http.Request) {
	nilCh <- 1
}
