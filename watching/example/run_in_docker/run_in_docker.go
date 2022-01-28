package main

import (
	"net/http"
	"time"

	"github.com/songzhibin97/gkit/watching"
)

func init() {
	http.HandleFunc("/docker", dockermake1gb)
	http.HandleFunc("/docker/cpu", cpuex)
	http.HandleFunc("/docker/cpu_multi_core", cpuMulticore)
	go http.ListenAndServe(":10003", nil)
}

func main() {
	w := watching.NewWatching(
		watching.WithCollectInterval("2s"),
		watching.WithCoolDown("1m"),
		watching.WithDumpPath("/tmp"),
		watching.WithTextDump(),
		watching.WithLoggerLevel(watching.LogLevelDebug),
		watching.WithMemDump(3, 25, 80),
		watching.WithCPUDump(60, 10, 80),
		watching.WithCGroup(true),
	)
	w.EnableCPUDump()
	w.EnableMemDump()
	w.Start()
	time.Sleep(time.Hour)
}

func cpuex(wr http.ResponseWriter, req *http.Request) {
	go func() {
		for {
		}
	}()
}

func cpuMulticore(wr http.ResponseWriter, req *http.Request) {
	for i := 1; i <= 100; i++ {
		go func() {
			for {
			}
		}()
	}
}

func dockermake1gb(wr http.ResponseWriter, req *http.Request) {
	a := make([]byte, 1073741824)
	_ = a
}
