package main

import (
	"fmt"

	"github.com/songzhibin97/gkit/distributed/example"
	"github.com/songzhibin97/gkit/distributed/task"
)

func main() {
	server := example.InitServer()
	if server == nil {
		panic("server is empty")
	}
	worker := server.NewWorker("worker", 0, server.GetConfig().ConsumeQueue)
	worker.SetErrorHandler(func(err error) {
		fmt.Println("I am an error handler:", err)
	})
	worker.SetBeforeTaskHandler(func(task *task.Signature) {
		fmt.Println("I am a start of task handler for:", task.Name)
	})

	worker.SetAfterTaskHandler(func(task *task.Signature) {
		fmt.Println("I am a end of task handler for:", task.Name)
	})
	err := worker.Start()
	fmt.Println("end worker:", err)
}
