package main

import (
	"net/http"
	"time"

	"github.com/songzhibin97/gkit/watching"
)

func init() {
	http.HandleFunc("/deadloop", deadloop)
	go http.ListenAndServe(":10003", nil)
}

func main() {
	w := watching.NewWatching(
		watching.WithCollectInterval("2s"),
		watching.WithCoolDown("1m"),
		watching.WithDumpPath("/tmp"),
		watching.WithCPUDump(10, 25, 80),
	)
	w.EnableCPUDump().Start()
	time.Sleep(time.Hour)
}

func deadloop(wr http.ResponseWriter, req *http.Request) {
	for i := 0; i < 4; i++ {
		for {
			time.Sleep(time.Millisecond)
		}
	}
}
