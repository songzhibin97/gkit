package controller_redis

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"

	"github.com/go-redis/redis/v8"
	"github.com/songzhibin97/gkit/distributed/broker"
	"github.com/songzhibin97/gkit/distributed/controller"
)

func InitController() controller.Controller {
	opt := redis.UniversalOptions{
		Addrs: []string{"127.0.0.1:6379"},
	}
	client := redis.NewUniversalClient(&opt)
	if client == nil {
		return nil
	}
	bk := broker.NewBroker(broker.NewRegisteredTask(), context.Background())
	go func() {
		n := 0
		for {
			time.Sleep(time.Second)
			if err := client.LPush(context.Background(), "test_task",
				fmt.Sprintf(`{
    "id":"%d",
    "name":"test_task"
}`, n)).Err(); err != nil {
				fmt.Println("err:", err)
			}
			n++
		}
	}()
	return NewControllerRedis(bk, client, "test_task", "delayed")
}

type processor struct{}

func (p processor) Process(t *task.Signature) error {
	fmt.Println("消费", t.ID, t.Name)
	return nil
}

func (p processor) ConsumeQueue() string {
	return "test_task"
}

func (p processor) PreConsumeHandler() bool {
	return false
}

func TestControllerRedis_StartConsuming(t *testing.T) {
	ct := InitController()
	ct.RegisterTask("test_task")
	go func() {
		time.Sleep(10 * time.Second)
		ct.StopConsuming()
	}()
	b, err := ct.StartConsuming(1, processor{})
	if err != nil {
		t.Error(err)
		return
	}
	t.Log(b)
}
