package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/songzhibin97/gkit/generator"

	"github.com/songzhibin97/gkit/distributed/example"
	"github.com/songzhibin97/gkit/distributed/task"
)

var generate = generator.NewSnowflake(time.Now(), 1)

func generateIDWithSignature(signature *task.Signature) *task.Signature {
	id, _ := generate.NextID()
	signature.ID = strconv.FormatUint(id, 10)
	return signature
}

func generateID() string {
	id, _ := generate.NextID()
	return strconv.FormatUint(id, 10)
}

func main() {
	server := example.InitServer()
	if server == nil {
		panic("server is empty")
	}
	var (
		addTask0, addTask1, addTask2                      task.Signature
		multiplyTask0, multiplyTask1                      task.Signature
		sumIntsTask, sumFloatsTask, concatTask, splitTask task.Signature
		panicTask                                         task.Signature
		longRunningTask                                   task.Signature
	)
	initTasks := func() {
		addTask0 = task.Signature{
			Name:   "add",
			Router: server.GetConfig().ConsumeQueue,
			Args: []task.Arg{
				{
					Type:  "int64",
					Value: int64(1),
				},
				{
					Type:  "int64",
					Value: int64(1),
				},
			},
		}

		addTask1 = task.Signature{
			Name:   "add",
			Router: server.GetConfig().ConsumeQueue,
			Args: []task.Arg{
				{
					Type:  "int64",
					Value: int64(2),
				},
				{
					Type:  "int64",
					Value: int64(2),
				},
			},
		}

		addTask2 = task.Signature{
			Name:   "add",
			Router: server.GetConfig().ConsumeQueue,
			Args: []task.Arg{
				{
					Type:  "int64",
					Value: int64(5),
				},
				{
					Type:  "int64",
					Value: int64(6),
				},
			},
		}

		multiplyTask0 = task.Signature{
			Name:   "multiply",
			Router: server.GetConfig().ConsumeQueue,
			Args: []task.Arg{
				{
					Type:  "int64",
					Value: int64(4),
				},
			},
		}

		multiplyTask1 = task.Signature{
			Name:   "multiply",
			Router: server.GetConfig().ConsumeQueue,
		}

		sumIntsTask = task.Signature{
			Name:   "sum_ints",
			Router: server.GetConfig().ConsumeQueue,
			Args: []task.Arg{
				{
					Type:  "[]int64",
					Value: []int64{1, 2},
				},
			},
		}

		sumFloatsTask = task.Signature{
			Name:   "sum_floats",
			Router: server.GetConfig().ConsumeQueue,
			Args: []task.Arg{
				{
					Type:  "[]float64",
					Value: []float64{1.5, 2.7},
				},
			},
		}

		concatTask = task.Signature{
			Name:   "concat",
			Router: server.GetConfig().ConsumeQueue,
			Args: []task.Arg{
				{
					Type:  "[]string",
					Value: []string{"foo", "bar"},
				},
			},
		}

		splitTask = task.Signature{
			Name:   "split",
			Router: server.GetConfig().ConsumeQueue,
			Args: []task.Arg{
				{
					Type:  "string",
					Value: "foo",
				},
			},
		}

		panicTask = task.Signature{
			Name:   "panic_task",
			Router: server.GetConfig().ConsumeQueue,
		}

		longRunningTask = task.Signature{
			Name:   "long_running_task",
			Router: server.GetConfig().ConsumeQueue,
		}
	}
	// 发送单个任务
	initTasks()

	asyncResult, err := server.SendTask(generateIDWithSignature(&addTask0))
	if err != nil {
		fmt.Println("Could not send task: ", err.Error())
		return
	}
	results, err := asyncResult.Get(time.Millisecond * 5)
	if err != nil {
		fmt.Println("Getting task result failed with error: ", err.Error())
		return
	}
	fmt.Println("1 + 1 =", task.HumanReadableResults(results))

	// 发送入参出参为slice的任务
	asyncResult, err = server.SendTask(generateIDWithSignature(&sumIntsTask))
	if err != nil {
		fmt.Println("Could not send task: ", err.Error())
		return
	}
	results, err = asyncResult.Get(time.Millisecond * 5)
	if err != nil {
		fmt.Println("Getting task result failed with error: ", err.Error())
		return
	}
	fmt.Println("sum([1, 2]) =", task.HumanReadableResults(results))

	asyncResult, err = server.SendTask(generateIDWithSignature(&sumFloatsTask))
	if err != nil {
		fmt.Println("Could not send task: ", err.Error())
		return
	}
	results, err = asyncResult.Get(time.Millisecond * 5)
	if err != nil {
		fmt.Println("Getting task result failed with error: ", err.Error())
		return
	}
	fmt.Println("sum([1.5, 2.7]) =", task.HumanReadableResults(results))

	asyncResult, err = server.SendTask(generateIDWithSignature(&concatTask))
	if err != nil {
		fmt.Println("Could not send task: ", err.Error())
		return
	}
	results, err = asyncResult.Get(time.Millisecond * 5)
	if err != nil {
		fmt.Println("Getting task result failed with error: ", err.Error())
		return
	}
	fmt.Println("concat([\"foo\", \"bar\"]) =", task.HumanReadableResults(results))

	asyncResult, err = server.SendTask(generateIDWithSignature(&splitTask))
	if err != nil {
		fmt.Println("Could not send task: ", err.Error())
		return
	}
	results, err = asyncResult.Get(time.Millisecond * 5)
	if err != nil {
		fmt.Println("Getting task result failed with error: ", err.Error())
		return
	}
	fmt.Println("split([\"foo\"]) =", task.HumanReadableResults(results))

	// 发送任务组
	initTasks()
	fmt.Println("Group of tasks (parallel execution):")
	group, err := task.NewGroup(generateID(), "group", generateIDWithSignature(&addTask0), generateIDWithSignature(&addTask1), generateIDWithSignature(&addTask2))
	if err != nil {
		fmt.Println("Error creating group:", err.Error())
		return
	}
	asyncResults, err := server.SendGroup(group, 10)
	if err != nil {
		fmt.Println("Could not send group:", err.Error())
	}
	for _, result := range asyncResults {
		results, err := result.Get(time.Millisecond * 5)
		if err != nil {
			fmt.Println("Getting task result failed with error:", err.Error())
			return
		}
		fmt.Println(result.Signature.Args[0].Value,
			result.Signature.Args[1].Value,
			task.HumanReadableResults(results))
	}

	// 发送具有回调的任务组
	group, err = task.NewGroup(generateID(), "group_with_callback", generateIDWithSignature(&addTask0), generateIDWithSignature(&addTask1), generateIDWithSignature(&addTask2))
	if err != nil {
		fmt.Println("Error creating group:", err.Error())
		return
	}
	callback, err := task.NewGroupCallback(group, "group_callback", generateIDWithSignature(&multiplyTask1))
	if err != nil {
		fmt.Println("Error creating group callback:", err)
		return
	}
	groupCallbackAsyncResult, err := server.SendGroupCallback(callback, 10)
	if err != nil {
		fmt.Println("Could not send group callback:", err.Error())
		return
	}
	results, err = groupCallbackAsyncResult.Get(time.Millisecond * 5)
	if err != nil {
		fmt.Println("Could not send group callback:", err.Error())
		return
	}
	fmt.Println("(1 + 1) * (2 + 2) * (5 + 6) = ", task.HumanReadableResults(results))

	// 链式任务
	initTasks()
	fmt.Println("Chain of tasks:")

	chain, err := task.NewChain("chain", generateIDWithSignature(&addTask0), generateIDWithSignature(&addTask1), generateIDWithSignature(&addTask2), generateIDWithSignature(&multiplyTask0))
	if err != nil {
		fmt.Println("Error creating chain:", err)
		return
	}
	chainAsyncResult, err := server.SendChain(chain)
	if err != nil {
		fmt.Println("Could not send chain:", err.Error())
		return
	}
	results, err = chainAsyncResult.Get(time.Millisecond * 5)
	if err != nil {
		fmt.Println("Getting chain result failed with error:", err.Error())
		return
	}
	fmt.Println("(((1 + 1) + (2 + 2)) + (5 + 6)) * 4 = ", task.HumanReadableResults(results))

	// panic 任务
	asyncResult, err = server.SendTask(generateIDWithSignature(&panicTask))
	if err != nil {
		fmt.Println("Could not send task: ", err.Error())
		return
	}
	_, err = asyncResult.Get(time.Millisecond * 5)
	if err == nil {
		fmt.Println("Error should not be nil if task panicked")
		return
	}
	fmt.Println("Task panicked and returned error = ", err.Error())

	// 长任务
	asyncResult, err = server.SendTask(generateIDWithSignature(&longRunningTask))
	if err != nil {
		fmt.Println("Could not send task: ", err.Error())
		return
	}
	results, err = asyncResult.Get(time.Millisecond * 5)
	if err != nil {
		fmt.Println("Getting long running task result failed with error: ", err.Error())
		return
	}
	fmt.Println("Long running task returned =", task.HumanReadableResults(results))
}
