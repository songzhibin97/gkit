package controller_redis

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"runtime"
	"strconv"
	"sync"
	"time"

	json "github.com/json-iterator/go"

	"github.com/songzhibin97/gkit/distributed/broker"

	"github.com/songzhibin97/gkit/distributed/task"

	"github.com/songzhibin97/gkit/distributed/controller"

	"github.com/go-redis/redis/v8"
	"github.com/go-redsync/redsync/v4"
	"github.com/go-redsync/redsync/v4/redis/goredis/v8"
)

type ControllerRedis struct {
	*broker.Broker
	// client redis客户端
	client redis.UniversalClient
	// lock 分布式锁
	lock *redsync.Redsync

	// wg

	// consumingWg 确保消费组并发完成
	consumingWg sync.WaitGroup
	// processingWg 确保正在处理的任务并发完成
	processingWg sync.WaitGroup
	// delayedWg 确保延时任务并发完成
	delayedWg sync.WaitGroup
	// consumingQueue 消费队列名称
	consumingQueue string
	// delayedQueue  延迟队列名称
	delayedQueue string
}

// SetConsumingQueue 设置消费队列名称
func (c *ControllerRedis) SetConsumingQueue(consumingQueue string) {
	c.consumingQueue = consumingQueue
}

// SetDelayedQueue 设置延迟队列名称
func (c *ControllerRedis) SetDelayedQueue(delayedQueue string) {
	c.delayedQueue = delayedQueue
}

func (c *ControllerRedis) RegisterTask(name ...string) {
	c.RegisterList(name...)
}

func (c *ControllerRedis) IsRegisterTask(name string) bool {
	return c.IsRegister(name)
}

func (c *ControllerRedis) StartConsuming(concurrency int, handler task.Processor) (bool, error) {
	c.consumingWg.Add(1)
	defer c.consumingWg.Done()

	// 设置阈值,如果并发数 < 1, 默认设置成 2*cpu
	if concurrency < 1 {
		concurrency = runtime.NumCPU() * 2
	}
	_, err := c.client.Ping(context.Background()).Result()
	if err != nil {
		// 重试
		c.Broker.GetRetryFn()(c.Broker.GetRetryCtx())
		if c.Broker.GetRetry() {
			return true, err
		}
		return false, controller.ErrorConnectClose
	}

	// 初始化工作区
	pool := make(chan struct{}, concurrency)
	worker := make(chan []byte, concurrency)

	// 填满并发池
	for i := 0; i < concurrency; i++ {
		pool <- struct{}{}
	}
	go func() {
		fmt.Println("[*] Waiting for messages. To exit press CTRL+C")
		for {
			select {
			case <-c.GetStopCtx().Done():
				close(worker)
				return
			case _, ok := <-pool:
				if !ok {
					return
				}
				tByte, err := c.popTask(c.consumingQueue, 0)
				if err != nil && !errors.Is(err, redis.Nil) {
					fmt.Println("popTask err:", err)
				}
				// 如果是有效数据,发送给worker
				if len(tByte) > 0 {
					worker <- tByte
				}
				pool <- struct{}{}
			}
		}
	}()
	c.delayedWg.Add(1)
	go func() {
		defer c.delayedWg.Done()
		for {
			select {
			case <-c.GetStopCtx().Done():
				return
			default:
				tBody, err := c.popDelayedTask(c.delayedQueue, 0)
				if err != nil {
					fmt.Println("popDelayedTask err:", err)
					continue
				}
				if tBody == nil {
					continue
				}
				t := task.Signature{}
				if err = json.Unmarshal(tBody, &t); err != nil {
					fmt.Println("unmarshal err:", err)
					continue
				}
				if err = c.Publish(c.GetStopCtx(), &t); err != nil {
					fmt.Println("publish err:", err)
					continue
				}
			}
		}
	}()

	if err = c.consume(worker, concurrency, handler); err != nil {
		return c.GetRetry(), err
	}
	c.processingWg.Wait()
	return c.GetRetry(), err
}

// popTask 弹出任务
func (c *ControllerRedis) popTask(queue string, blockTime int64) ([]byte, error) {
	if blockTime <= 0 {
		blockTime = int64(1000 * time.Millisecond)
	}
	items, err := c.client.BLPop(context.Background(), time.Duration(blockTime), queue).Result()
	if err != nil {
		return nil, err
	}
	// items[0] - the name of the key where an element was popped
	// items[1] - the value of the popped element
	if len(items) != 2 {
		return nil, redis.Nil
	}
	result := []byte(items[1])
	return result, nil
}

// popDelayedTask 弹出延时任务,因为延时任务是使用Redis ZSet
func (c *ControllerRedis) popDelayedTask(queue string, blockTime int64) ([]byte, error) {
	if blockTime <= 0 {
		blockTime = int64(1000 * time.Millisecond)
	}
	var result []byte
	for {
		time.Sleep(time.Duration(blockTime))
		watchFn := func(tx *redis.Tx) error {
			now := time.Now().Local().UnixNano()
			max := strconv.FormatInt(now, 10)
			items, err := tx.ZRevRangeByScore(c.GetStopCtx(), queue, &redis.ZRangeBy{Min: "0", Max: max, Offset: 0, Count: 1}).Result()
			if err != nil {
				return err
			}
			if len(items) != 1 {
				return redis.Nil
			}
			_, err = tx.TxPipelined(c.GetStopCtx(), func(pipe redis.Pipeliner) error {
				pipe.ZRem(c.GetStopCtx(), queue, items[0])
				result = []byte(items[0])
				return nil
			})
			return err
		}
		if err := c.client.Watch(c.GetStopCtx(), watchFn, queue); err != nil {
			break
		}
	}
	return result, nil
}

// consume 消费
func (c *ControllerRedis) consume(worker <-chan []byte, concurrency int, handler task.Processor) error {
	// 初始化工作区
	pool := make(chan struct{}, concurrency)
	errorsChan := make(chan error, concurrency*2)

	// 填满并发池
	for i := 0; i < concurrency; i++ {
		pool <- struct{}{}
	}
	for {
		select {
		case <-c.GetStopCtx().Done():
			return c.GetStopCtx().Err()
		case err := <-errorsChan:
			return err
		case v, ok := <-worker:
			if !ok {
				return nil
			}
			// 阻塞等待
			select {
			case <-pool:
			case <-c.GetStopCtx().Done():
				return c.GetStopCtx().Err()
			}
			c.processingWg.Add(1)
			go func() {
				if err := c.consumeOne(v, c.consumingQueue, handler); err != nil {
					errorsChan <- err
				}
				c.processingWg.Done()

				pool <- struct{}{}
			}()
		}
	}
}

func (c *ControllerRedis) consumeOne(taskBody []byte, queue string, handler task.Processor) error {
	t := task.Signature{}
	decoder := json.NewDecoder(bytes.NewReader(taskBody))
	decoder.UseNumber()
	if err := decoder.Decode(&t); err != nil {
		return err
	}

	if !c.IsRegisterTask(t.Name) {
		if t.IgnoreNotRegisteredTask {
			return nil
		}
		c.client.RPush(c.GetStopCtx(), queue, handler)
		return nil
	}
	return handler.Process(&t)
}

func (c *ControllerRedis) StopConsuming() {
	c.Broker.StopConsuming()
	c.consumingWg.Wait()
	c.delayedWg.Wait()
	c.client.Close()
}

func (c *ControllerRedis) Publish(ctx context.Context, t *task.Signature) error {
	tBody, err := json.Marshal(t)
	if err != nil {
		return err
	}
	if t.ETA != nil {
		now := time.Now().Local()
		if t.ETA.After(now) {
			score := t.ETA.UnixNano()
			return c.client.ZAdd(c.GetStopCtx(), c.delayedQueue, &redis.Z{Score: float64(score), Member: tBody}).Err()
		}
	}
	return c.client.RPush(c.GetStopCtx(), t.Router, tBody).Err()
}

func (c *ControllerRedis) GetPendingTasks(queue string) ([]*task.Signature, error) {
	results, err := c.client.LRange(c.GetStopCtx(), queue, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	taskSlice := make([]*task.Signature, 0, len(results))
	for _, result := range results {
		var t task.Signature
		err = json.Unmarshal([]byte(result), &t)
		if err != nil {
			return nil, err
		}
		taskSlice = append(taskSlice, &t)
	}
	return taskSlice, nil
}

func (c *ControllerRedis) GetDelayedTasks() ([]*task.Signature, error) {
	results, err := c.client.LRange(c.GetStopCtx(), c.delayedQueue, 0, -1).Result()
	if err != nil {
		return nil, err
	}
	taskSlice := make([]*task.Signature, 0, len(results))
	for _, result := range results {
		var t task.Signature
		err = json.Unmarshal([]byte(result), &t)
		if err != nil {
			return nil, err
		}
		taskSlice = append(taskSlice, &t)
	}
	return taskSlice, nil
}

func NewControllerRedis(broker *broker.Broker, client redis.UniversalClient, consumingQueue, delayedQueue string) controller.Controller {
	return &ControllerRedis{
		Broker:         broker,
		client:         client,
		lock:           redsync.New(goredis.NewPool(client)),
		consumingQueue: consumingQueue,
		delayedQueue:   delayedQueue,
	}
}
