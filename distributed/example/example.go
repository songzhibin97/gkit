package example

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/songzhibin97/gkit/distributed/backend/backend_redis"

	"github.com/songzhibin97/gkit/distributed/backend"
	zlog "github.com/songzhibin97/gkit/log"

	"github.com/go-redis/redis/v8"

	"github.com/songzhibin97/gkit/distributed"
	"github.com/songzhibin97/gkit/distributed/broker"
	"github.com/songzhibin97/gkit/distributed/controller/controller_redis"
	"github.com/songzhibin97/gkit/distributed/locker/lock_ridis"
)

// Add ...
func Add(args ...int64) (int64, error) {
	sum := int64(0)
	for _, arg := range args {
		sum += arg
	}
	return sum, nil
}

// Multiply ...
func Multiply(args ...int64) (int64, error) {
	sum := int64(1)
	for _, arg := range args {
		sum *= arg
	}
	return sum, nil
}

// SumInts ...
func SumInts(numbers []int64) (int64, error) {
	var sum int64
	for _, num := range numbers {
		sum += num
	}
	return sum, nil
}

// SumFloats ...
func SumFloats(numbers []float64) (float64, error) {
	var sum float64
	for _, num := range numbers {
		sum += num
	}
	return sum, nil
}

// Concat ...
func Concat(strs []string) (string, error) {
	var res string
	for _, s := range strs {
		res += s
	}
	return res, nil
}

// Split ...
func Split(str string) ([]string, error) {
	return strings.Split(str, ""), nil
}

// PanicTask ...
func PanicTask() (string, error) {
	panic(errors.New("oops"))
}

// LongRunningTask ...
func LongRunningTask() error {
	fmt.Println("Long running task started")
	for i := 0; i < 10; i++ {
		fmt.Println(10 - i)
		time.Sleep(1 * time.Second)
	}
	fmt.Println("Long running task finished")
	return nil
}

func InitServer() *distributed.Server {
	opt := redis.UniversalOptions{
		Addrs: []string{"127.0.0.1:6379"},
	}
	client := redis.NewUniversalClient(&opt)
	if client == nil {
		return nil
	}
	lock := lock_ridis.NewRedisLock(client)
	bk := broker.NewBroker(broker.NewRegisteredTask(), context.Background())
	c := controller_redis.NewControllerRedis(bk, client, "gkit:queue", "delayed")

	var backendClient backend.Backend
	{
		// redis
		backendClient = backend_redis.NewBackendRedis(client, -1)
	}
	//{
	//	// mongodb
	//	mongoClient, err := mongo.NewClient()
	//	if err != nil {
	//		return nil
	//	}
	//	err = mongoClient.Connect(context.Background())
	//	if err != nil {
	//		return nil
	//	}
	//	backendClient = backend_mongodb.NewBackendMongoDB(mongoClient, -1)
	//}
	{
		//dsn := "root:123456@tcp(127.0.0.1:3306)/gkit?charset=utf8mb4&parseTime=True&loc=Local"
		//sqlDB, err := sql.Open("mysql", dsn)
		//if err != nil {
		//	return nil
		//}
		//backendClient = backend_db.NewBackendSQLDB(sqlDB, -1, "mysql", &gorm.Config{
		//	Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
		//		SlowThreshold:             time.Second,
		//		Colorful:                  false,
		//		IgnoreRecordNotFoundError: true,
		//		LogLevel:                  logger.Error,
		//	}),
		//})
	}

	// Register tasks
	tasksMap := map[string]interface{}{
		"add":               Add,
		"multiply":          Multiply,
		"sum_ints":          SumInts,
		"sum_floats":        SumFloats,
		"concat":            Concat,
		"split":             Split,
		"panic_task":        PanicTask,
		"long_running_task": LongRunningTask,
	}
	s := distributed.NewServer(c, backendClient, lock, zlog.NewHelper(zlog.With(zlog.DefaultLogger)), nil)
	err := s.RegisteredTasks(tasksMap)
	if err != nil {
		panic(err)
	}
	return s
}
