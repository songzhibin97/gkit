# GKIT

```
_____/\\\\\\\\\\\\__/\\\________/\\\__/\\\\\\\\\\\__/\\\\\\\\\\\\\\\_        
 ___/\\\//////////__\/\\\_____/\\\//__\/////\\\///__\///////\\\/////__       
  __/\\\_____________\/\\\__/\\\//_________\/\\\___________\/\\\_______      
   _\/\\\____/\\\\\\\_\/\\\\\\//\\\_________\/\\\___________\/\\\_______     
    _\/\\\___\/////\\\_\/\\\//_\//\\\________\/\\\___________\/\\\_______    
     _\/\\\_______\/\\\_\/\\\____\//\\\_______\/\\\___________\/\\\_______   
      _\/\\\_______\/\\\_\/\\\_____\//\\\______\/\\\___________\/\\\_______  
       _\//\\\\\\\\\\\\/__\/\\\______\//\\\__/\\\\\\\\\\\_______\/\\\_______ 
        __\////////////____\///________\///__\///////////________\///________                                 
```
 
##### Translate to: [简体中文](README_zh.md)

# Project Description
Dedicated to providing microservices and monolithic services of the availability of a collection of basic component tools, drawing on some of the best open source projects such as : `kratos`, `go-kit`, `mosn`, `sentinel`, `gopkg` ... We hope you will support us!

# Directory structure
```shell
├── cache (builds cache-related components)
  ├── buffer (provides byte array reuse and io buffer wrapping)
  ├── mbuffer (buffer-like implementation) 
  ├── local_cache (provides local key-value wrapper implementation for building local caches)
  ├── singleflight (provides prevention of duplicate tasks in high concurrency situations, generally used to fill cache miss scenarios)
├── coding (provides object serialization/deserialization interface, provides json, proto, xml, yaml instance methods)
├── concurrent (best practices for using channels in concurrency)
  ├── fan_in (fan-in pattern, commonly used with multiple producers and one consumer in the producer-consumer model)
  ├── fan_out (fan-out mode, often used with a producer-consumer model where there are multiple producers and multiple consumers)
  ├── or_done (a concurrency scenario in which any one task is returned immediately after completion)
  ├── orderly (keep orderly completion even in concurrent scenarios)
  ├── map_reduce 
  ├── stream (provides data production stream encapsulation, and implementation of processing streams)
  ├── pipeline (concurrency becomes serial)
├── container (containerized component, providing groups, pools, queues)
  ├── group (provides a lazy loading mode for containers, similar to sync.Pool, which uses a key to get the corresponding container instance when used, or generates it if it doesn\'t exist)
  ├── pool (provides a wrapped abstraction of pool, and an implementation of the interface using lists)
  ├── queue
    ├── codel (implements a controlled delay algorithm for columns, and sanctions backlogged tasks)
├── delayed (delayed tasks - standalone version)
├── distributed (distributed tasks, provides standardized interfaces and corresponding implementations for redis, mysql, pgsql, mongodb)
├── downgrade (fusion downgrade related components)
├── egroup (errgroup, controls component lifecycle)
├── errors (grpc error handling)
├── gctuner (pre go1.19 gc optimization tool)
├── generator (number generator, snowflake)
├── goroutine (provide goroutine pools, control goroutine spikes)
├── log (interface logging, use logging component to access)
├── metrics (interface to metrics)
├── middleware (middleware interface model definition)
├── net (network related encapsulation)
  ├── tcp
├── options (option model interfacing)
├── overload (server adaptive protection, provides bbr interface, monitors deployed server status to select traffic release, protects server availability)
  ├── bbr (adaptive flow limiting)
├── page_token (google aip next token implementation)  
├── parser (file parsing, proto<->go mutual parsing)
  ├── parseGo (parses go to generate pb)
  ├── parsePb (parses pb to generate go)
├── registry (service discovery interfacing, google sre subset implementation)
├── restrictor (restrict flow, provide token bucket and leaky bucket interface wrappers)
  ├── client_throttling (client throttling)
  ├── rate 
  ├── ratelimite 
├── structure (common data structure)
  ├── hashset (hash tables)
  ├── lscq (lock-free unbounded queue, supports arm)
  ├── skipmap 
  ├── skipset 
  ├── zset 
├── sync
    ├── cpu (Get system information for Linux, including cpu mains, cpu usage, etc.)
    ├── fastrand (random numbers)
    ├── goid (get goroutine id)
    ├── mutex (provides trylock, reentrant lock and token reentrant lock)
    ├── nanotime (timestamp optimization)
    ├── once (a more powerful implementation of once, set the once function to return an error, and retry if it fails)
    ├── queue (lock-free queue)
    ├── safe (underlying string, slice structure)
    ├── stringx (enhanced version of string)
    ├── syncx (sync enhancement)
    ├── xxhash3 
├── ternary (ternary expressions)    
├── timeout (timeout control, full link protection, provides some encapsulated implementation of database processing time)
  ├── ctime (link timeout control)
  ├── c_json (adapt to database json types)
  ├── d_time (adapts to database to store time only)
  ├── date (Adapts database to store dates only)
  ├── date_struct (Adapts database to store dates only)
  ├── datetime (adapter database stores datetime)
  ├── datetime_struct (adapter database stores datetime)
  ├── stamp (adapter database stores timestamps)
  ├── human (provides visual time spacing)
├── tools 
  ├── bind (binding tool, often used with the gin framework to customize the bound data, e.g. binding both query and json)
  ├── deepcopy (deep copy)
  ├── float (floating point truncation tool)
  ├── match (base matcher, match on wildcards)
  ├── pointer (pointer tool)
  ├── pretty (formatting json)
  ├── reflect2value (basic field mapping)
  ├── rand_string (random strings)
  ├── vto (assignment of functions with the same type, hands free, usually used for vo->do object conversions)
    ├── vtoPlus (adds plus support for field, tag and default value binding)
├── trace (link tracing)
├── watching (monitor cpu, mum, gc, goroutine and other metrics, automatically dump pprof metrics in case of fluctuations)
└── window (sliding window, supports multi-data type metrics window collection)

```

# Download and use
```shell
go get github.com/songzhibin97/gkit
```


## Introduction to the use of components
## cache

Cache-related components
> buffer & mbuffer provide similar functionality, buffer has more encapsulation and implements some interfaces to io, while mbuffer is just a memory cache; it is more suitable for short and frequent life cycles.
> local_cache provides a local data cache, and also has some expiry mechanisms, you can set the expiry time, and regularly clean up the expired data, but he is now older, if needed there is a generic version https://github.com/songzhibin97/go-baseutils/blob/main/ app/bcache
> singleflight wraps golang.org/x/sync/singleflight to prevent the effects of changes.


### buffer pool
```go
package main

import (
	"fmt"
	"github.com/songzhibin97/gkit/cache/buffer"
)

func main() {
	// Byte reuse

	// size 2^6 - 2^18
	// return an integer multiple of 2 rounded upwards cap, len == size
	// Any other special or expanded during runtime will be cleared
	slice := buffer.GetBytes(1024)
	fmt.Println(len(*slice), cap(*slice)) // 1024 1024

	// Recycle
	// Note: no further references are allowed after recycling
	buffer.PutBytes(slice)

	// IOByte reuse

	// io buffer.IoBuffer interface
	GetIoPool(1024)

	// If an object has already been recycled, referring to the recycled object again will trigger an error
	err := buffer.PutIoPool(io)
	if err != nil {
		// Handle the error 	    
	}
}
```


### local_cache
```go
package local_cache

import (
	"github.com/songzhibin97/gkit/cache/buffer"
	"log"
)

var ch Cache

func ExampleNewCache() {
	// default configuration
	// ch = NewCache()

	// Optional configuration options

	// Set the interval time
	// SetInternal(interval time.Duration)

	// Set the default timeout
	// SetDefaultExpire(expire time.Duration)

	// Set the cycle execution function, the default (not set) is to scan the global to clear expired k
	// SetFn(fn func())

	// Set the capture function to be called after the deletion is triggered, the set capture function will be called back after the data is deleted
	// SetCapture(capture func(k string, v interface{}))

	// Set the initialization of the stored member object
	// SetMember(m map[string]Iterator)

	ch = NewCache(SetInternal(1000).
		SetDefaultExpire(10000).
		SetCapture(func(k string, v interface{}) {
			log.Println(k, v)
		}))
}

func ExampleCacheStorage() {
	// Set adds cache and overwrites it whether it exists or not
	ch.Set("k1", "v1", DefaultExpire)

	// SetDefault overrides whether or not it exists
	// Default function mode, default timeout is passed in as the default time to create the cache
	ch.SetDefault("k1", 1)

	// SetNoExpire
	// partial function mode, default timeout is never expired
	ch.SetNoExpire("k1", 1.1)

	// Add the cache and throw an exception if it exists
	err := ch.Add("k1", nil, DefaultExpire)
	CacheErrExist(err) // true

	// Replace throws an error if it is set or not
	err = ch.Replace("k2", make(chan struct{}), DefaultExpire)
	CacheErrNoExist(err) // true
}

func ExampleGet() {
	// Get the cache based on the key to ensure that kv is removed within the expiration date
	v, ok := ch.Get("k1")
	if !ok {
		// v == nil
	}
	_ = v

	// GetWithExpire gets the cache based on the key and brings up the timeout
	v, t, ok := ch.GetWithExpire("k1")
	if !ok {
		// v == nil
	}
	// if the timeout is NoExpire t.IsZero() == true
	if t.IsZero() {
		// No timeout is set
	}

	// Iterator returns all valid objects in the cache
	mp := ch.Iterator()
	for s, iterator := range mp {
		log.Println(s, iterator)
	}
	
	// Count returns the number of members
	log.Println(ch.Count())
}

func ExampleIncrement() {
	ch.Set("k3", 1, DefaultExpire)
	ch.Set("k4", 1.1, DefaultExpire)
	// Increment adds n to the value corresponding to k n must be a number type
	err := ch.Increment("k3", 1)
	if CacheErrExpire(err) || CacheErrExist(CacheTypeErr) {
		// Not set successfully
	}
	_ = ch.IncrementFloat("k4", 1.1)

	// If you know the exact type of k to set you can also use the type-determining increment function
	// ch.IncrementInt(k string, v int)
	// ...
	// IncrementFloat32(k string, v flot32) // ch.
	// ...

	// Decrement the same
}

func ExampleDelete() {
	// Delete triggers the not-or function if capture is set
	ch.Delete("k1")

	// DeleteExpire Deletes all expired keys, the default capture is to execute DeleteExpire()
	ch.DeleteExpire()
}

func ExampleChangeCapture() {
	// Provides a way to change the capture function on the fly
	// ChangeCapture
	ch.ChangeCapture(func(k string, v interface{}) {
		log.Println(k, v)
	})
}

func ExampleSaveLoad() {
	// Write to a file using go's proprietary gob protocol

	io := buffer.NewIoBuffer(1000)

	// Save pass in a w io.Writer argument to write the member members of the cache to w
	_ = ch.Save(io)

	// SaveFile pass in a path to write to a file
	_ = ch.SaveFile("path")

	// Load pass in an r io.Reader object read from r and write back to member
	_ = ch.Load(io)

	// LoadFile pass in path to read the contents of the file
	_ = ch.LoadFile("path")
}

func ExampleFlush() {
	// Flush to free member members
	ch.Flush()
}

func ExampleShutdown() {
	// Shutdown frees the object
	ch.Shutdown()
}
```

### singleflight

Merge back to source
```go
package main

import (
	"github.com/songzhibin97/gkit/cache/singleflight"
)

// getResources: Generally used to go to the database and fetch data
func getResources() (interface{}, error) {
	return "test", nil
}

// cache: populate the cache with data
func cache(v interface{}) {
	return
}

func main() {
	f := singleflight.NewSingleFlight()

	// Synchronize.
	v, err, _ := f.Do("test1", func() (interface{}, error) {
		// Get resources
		return getResources()
	})
	if err != nil {
		// Handle the error
	}
	// store to buffer
	// v is the fetched resource
	cache(v)

	// asynchronously
	ch := f.DoChan("test2", func() (interface{}, error) {
		// Get resources
		return getResources()
	})

	// Wait for the resource to be fetched, then return the result via channel
	result := <-ch
	if result.Err != nil {
		// Handle the error
	}
	
	// store to buffer
	// result.Val is the fetched resource
	cache(result.Val)
	
	// try to cancel
	f.Forget("test2")
}
```


## coding
> Object serialization deserialization interface and instance encapsulation, just import anonymous, such as json `_ "github.com/songzhibin97/gkit/coding/json"` You can also implement the corresponding interface, after registration, the benefit is to control the global serialization object and the use of library unity

```go
package main

import (
	"fmt"
	"github.com/songzhibin97/gkit/coding"
	_ "github.com/songzhibin97/gkit/coding/json" // Be sure to import ahead of time!!!
)

func main() {
	t := struct {
		Gkit string
		Lever int
	}{"Gkit", 200}
	fmt.Println(coding.GetCode("json").Name())
	GetCode("json").Marshal(t)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(data)) // {"Gkit": "Gkit", "Lever":200}
	v := struct {
		Gkit string
		Lever int
	}{}
	coding.GetCode("json").Unmarshal(data,&v)
	fmt.Println(v) // { Gkit 200}
}
```

## concurrent

> Best practices for channel in concurrency, including fan_in,fan_out,map_reduce,or_done,orderly,pipeline,stream,generic version can be found at https://github.com/songzhibin97/go-baseutils/tree/main/app/bconcurrent

## container

> Pool, but you can customize the creation function and replace the initialization function.
> pool pooling object, through the configuration can set the maximum number of connections and the number of waiting connections, synchronous asynchronous acquisition of connections, and connection life cycle management, you can customize the creation function, and replace the initialization function
> codel implement codel algorithm, can be used to limit the flow, and fuse

### group

Lazy loading containers
```go
package main

import (
	"fmt"
	"github.com/songzhibin97/gkit/container/group"
)

func createResources() interface{} {
	return map[int]int{1: 1, 2: 2}
}

func createResources2() interface{} {
	return []int{1, 2, 3}
}

func main() {
	// Like sync.Pool
	// initialize a group
	g := group.NewGroup(createResources)

	// If the key does not exist call the function passed in by NewGroup to create resources
	// If it exists, return the information about the created resource
	v := g.Get("test")
	fmt.Println(v) // map[1:1 2:2]
	v.(map[int]int)[1] = 3
	fmt.Println(v) // map[1:3 2:2]
	v2 := g.Get("test")
	fmt.Println(v2) // map[1:3 2:2]

	// ReSet resets the initialization function and clears the cached key
	g.ReSet(createResources2)
	v3 := g.Get("test")
	fmt.Println(v3) // []int{1,2,3}
	
	// Clear the cached buffer
	g.Clear()
}
```

### pool

Similar to a resource pool
```go
package main

import (
	"context"
	"fmt"
	"github.com/songzhibin97/gkit/container/pool"
	"time"
)

var p pool.Pool

type mock map[string]string

func (m *mock) Shutdown() error {
	return nil
}

// getResources: gets resources, returns a resource object that needs to implement the IShutdown interface for resource recycling
func getResources(context.Context) (pool.IShutdown, error) {
	return &mock{"mockKey": "mockValue"}, nil
}

func main() {
	// pool.NewList(options ...)
	// default configuration
	// p = pool.NewList()

	// Optional configuration options

	// Set the number of Pool connections, if == 0 then no limit
	// pool.SetActive(100)

	// Set the maximum number of free connections
	// pool.SetIdle(20)

	// Set idle wait time
	// pool.SetIdleTimeout(time.Second)

	// Set the expected wait
	// pool.SetWait(false,time.Second)

	// Customize the configuration
	p = pool.NewList(
		pool.SetActive(100).
		pool.SetIdle(20).
		pool.SetIdleTimeout(time.Second).
		SetIdleTimeout(time.Second), pool.SetWait(false, time.Second))

	// New needs to be instantiated, otherwise it will not get the resource in pool.
	p.NewQueue(getResources)

	v, err := p.Get(context.TODO())
	if err != nil {
		// Handle the error
	}
	fmt.Println(v) // &map[mockKey:mockValue]

	// Put: Resource recovery
	// forceClose: true internally help you call Shutdown to recycle, otherwise determine if it is recyclable and mount it on a list
	err = p.Put(context.TODO(), v, false)
	if err != nil {
		// handle the error  	    
	}
	
	// Shutdown reclaims resources, shutting down all resources
	_ = p.Shutdown()
}
```


### queue

codel implementation

```go
package main

import (
	"context"
	"fmt"
	"github.com/songzhibin97/gkit/container/queue/codel"
	"github.com/songzhibin97/gkit/overload/bbr"
)

func main() {
	queue := codel.NewQueue(codel.SetTarget(40), codel.SetInternal(1000))

	// start reflects CoDel state information
	start := queue.Stat()
	fmt.Println(start)

	go func() {
		// where the actual consumption takes place
		queue.Pop()
	}()
	if err := queue.Push(context.TODO()); err != nil {
		if err == bbr.LimitExceed {
			// todo handles overload protection errors
		} else {
			// todo handles other errors
		}
	}
}
```

## delayed

Delayed tasks (stand-alone version)

```go
package main

import "github.com/songzhibin97/gkit/delayed"

type mockDelayed struct {
	exec int64
}

func (m mockDelayed) Do() {
}

func (m mockDelayed) ExecTime() int64 {
	return m.exec
}

func (m mockDelayed) Identify() string {
	return "mock"
}

func main() {
	// Create a delayed object
	// delayed.SetSingle() sets the monitor signal
	// delayed.SetSingleCallback() set the signal callback
	// delayed.SetWorkerNumber() sets the worker concurrently
	// delayed.SetCheckTime() Set the monitoring time
	n := delayed.NewDispatchingDelayed()

	// add a delayed task
	n.AddDelayed(mockDelayed{exec: 1})
	
	// Force a refresh
	n.Refresh()
	
	// Close
	n.Close()
}

```

## distributed 

Distributed tasks (see test cases for detailed use)

## downgrade

Meltdown downgrade

```go
// Same as github.com/afex/hystrix-go, but in an abstract wrapper to avoid the impact of upgrades on the service
package main

import (
	"context"
	"github.com/afex/hystrix-go/hystrix"
	"github.com/songzhibin97/gkit/downgrade"
)

var fuse downgrade.Fuse

type RunFunc = func() error
type FallbackFunc = func(error) error
type RunFuncC = func(context.Context) error
type FallbackFuncC = func(context.Context, error) error

var outCH = make(chan struct{}, 1)

func mockRunFunc() RunFunc {
	return func() error {
		outCH <- struct{}{}
		return nil
	}
}

func mockFallbackFunc() FallbackFunc {
	return func(err error) error {
		return nil
	}
}

func mockRunFuncC() RunFuncC {
	return func(ctx context.Context) error {
		return nil
	}
}

func mockFallbackFuncC() FallbackFuncC {
	return func(ctx context.Context, err error) error {
		return nil
	}
}

func main() {
	// Get a fuse
	fuse = downgrade.NewFuse()

	// Leave ConfigureCommand unset and go to the default configuration
	// hystrix.CommandConfig{} set the parameters
	fuse.ConfigureCommand("test", hystrix.CommandConfig{})

	// Do: execute func() error synchronously, no timeout control until it returns.
	// if return error ! = nil then FallbackFunc is triggered to downgrade
	err := fuse.Do("do", mockRunFunc(), mockFallbackFunc())
	if err != nil {
		// Handle the error
	}

	// Go: asynchronous execution Return channel
	ch := fuse.Go("go", mockRunFunc(), mockFallbackFunc())
	select {
	case err = <-ch:
	// Handle the error
	case <-outCH:
		break
	}

	// GoC: Do/Go actually ends up calling GoC, the Do master handles the asynchronous process
	// GoC can be passed into context to ensure link timeout control
	fuse.GoC(context.TODO(), "goc", mockRunFuncC(), mockFallbackFuncC())
}
```

## egroup

Component Lifecycle Management

```go
// errorGroup 
// Cascade control, if a component has an error, all components in the group will be notified to exit
// Declare lifecycle management

package main

import (
	"context"
	"github.com/songzhibin97/gkit/egroup"
	"os"
	"syscall"
	"time"
)

var admin *egroup.LifeAdmin

func mockStart() func(ctx context.Context) error {
	return nil
}

func mockShutdown() func(ctx context.Context) error {
	return nil
}

type mockLifeAdminer struct{}

func (m *mockLifeAdminer) Start(ctx context.Context) error {
	return nil
}

func (m *mockLifeAdminer) Shutdown(ctx context.Context) error {
	return nil
}

func main() {
	// Default configuration
	// admin = egroup.NewLifeAdmin()

	// Optional configuration options

	// set the start timeout

	// <= 0 no start timeout, note that shutdown notifications should be handled at shutdown
	// egroup.SetStartTimeout(time.Second)

	// Set the shutdown timeout
	// <=0 no start timeout
	// egroup.SetStopTimeout(time.Second)

	// Set the set of signals, and the functions to handle them
	// egroup.SetSignal(func(lifeAdmin *LifeAdmin, signal os.Signal) {
	// return
	// }, signal...)

	admin = egroup.NewLifeAdmin(egroup.SetStartTimeout(time.Second), egroup.SetStopTimeout(time.Second),
		egroup.SetSignal(func(a *egroup.LifeAdmin, signal os.Signal) {
			switch signal {
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
				a.Shutdown()
			default:
			}
		}))
	
	// Add via struct
	admin.Add(egroup.Member{
		Start:    mockStart(),
		Shutdown: mockShutdown(),
	})

	// Adapted via interface
	admin.AddMember(&mockLifeAdminer{})

	// Start
	defer admin.Shutdown()
	if err := admin.Start(); err != nil {
		// Handle errors
		// normal start will hold main
	}
}

func Demo() {
	// Full demo
	var _admin = egroup.NewLifeAdmin()

	srv := &http.Server{
		Addr: ":8080",
	}
	// Add a task
	_admin.Add(egroup.Member{
		Start: func(ctx context.Context) error {
			fmt.Println("http start")
			return goroutine.Delegate(ctx, -1, func(ctx context.Context) error {
				return srv.ListenAndServe()
			})
		},
		Shutdown: func(ctx context.Context) error {
			fmt.Println("http shutdown")
			return srv.Shutdown(context.Background())
		},
	})
	// _admin.Start() start
	fmt.Println("error", _admin.Start())
	defer _admin.Shutdown()
}

```

## errors

Wrapping some error handling

```go
package main

import (
	"fmt"
	"net/http"
	"time"
	
	"github.com/songzhibin97/gkit/errors"
)
func main() {
	err := errors.Errorf(http.StatusBadRequest, "reason", "carrying message %s", "test")
	err2 := err.AddMetadata(map[string]string{"time": time.Now().String()}) // Carry meta information
	// err is the original error err2 is the error with meta information
	fmt.Println(errors.Is(err,err2)) // ture
	// err2 can be parsed to get more information
	fmt.Println(err2.Metadata["time"]) // meta
}
````

## gctuner

```go

// Get mem limit from the host machine or cgroup file.
limit := 4 * 1024 * 1024 * 1024
threshold := limit * 0.7

gctuner.Tuning(threshold)

// Friendly input
gctuner.TuningWithFromHuman("1g")

// Auto
// There may be problems with multiple services in one pod.
gctuner.TuningWithAuto(false) // Is it a container? Incoming Boolean
```

## generator

Generator

### snowflake

Snowflake algorithm

```go
package main

import (
	"fmt"
	"github.com/songzhibin97/gkit/generator"
	"time"
)

func main() {
	ids := generator.NewSnowflake(time.Now(), 1)
	nid, err := ids.NextID()
	if err != nil {
		// Handle the error
	}
	fmt.Println(nid)
}

```

## goroutine

Pooling, controlling wild goroutines

```go
package main

import (
	"context"
	"fmt"
	"github.com/songzhibin97/gkit/goroutine"
	"time"
)

var gGroup goroutine.

func mockFunc() func() {
	return func() {
		fmt.Println("ok")
	}
}

func main() {
	// Default configuration
	// gGroup = goroutine.NewGoroutine(context.TODO())

	// Optional configuration options

	// set the stop timeout time
	// goroutine.SetStopTimeout(time.Second)

	// Set the log object
	// goroutine.SetLogger(&testLogger{})

	// Set the maximum pool size
	// goroutine.SetMax(100)

	gGroup = goroutine.NewGoroutine(context.TODO(),
		goroutine.SetStopTimeout(time.Second),
		goroutine.SetMax(100),
	)

	// Add a task
	if !gGroup.AddTask(mockFunc()) {
		// Adding a task failed
	}

	// Add a task with timeout control
	if !gGroup.AddTaskN(context.TODO(), mockFunc()) {
		// Failed to add task
	}

	// Modify the pool maximum size
	gGroup.ChangeMax(1000)

	// Recycle resources
	_ = gGroup.Shutdown()
}
```

## log

Logging related

```go
package main

import (
	"fmt"
	"github.com/songzhibin97/gkit/log"
)

type testLogger struct{}

func (l *testLogger) Print(kv ...interface{}) {
	fmt.Println(kv...)
}

func main() {
	logs := log.NewHelper(log.DefaultLogger)
	logs.Debug("debug", "v")
	logs.Debugf("%s,%s", "debugf", "v")
	logs.Info("Info", "v")
	logs.Infof("%s,%s", "infof", "v")
	logs.Warn("Warn", "v")
	logs.Warnf("%s,%s", "warnf", "v")
	logs.Error("Error", "v")
	logs.Errorf("%s,%s", "errorf", "v")
	/*
	[debug] message=debugv
    [debug] message=debugf,v
    [Info] message=Infov
    [Info] message=infof,v
    [Warn] message=Warnv
    [Warn] message=warnf,v
    [Error] message=Errorv
    [Error] message=errorf,v
	*/
	
	logger := log.DefaultLogger
	logger = log.With(logger, "ts", log.DefaultTimestamp, "caller", log.DefaultCaller)
	logger.Log(log.LevelInfo, "msg", "helloworld")
	// [Info] ts=2021-06-10T13:41:35+08:00 caller=main.go:8 msg=helloworld
}
```

## metrics

Provides a metrics interface for implementing monitoring configurations
```go
package main

type Counter interface {
	With(lvs ...string) Counter
	Inc()
	Add(delta float64)
}

// Gauge is metrics gauge.
type Gauge interface {
	With(lvs ...string) Gauge
	Set(value float64)
	Add(delta float64)
	Sub(delta float64)
}

// Observer is metrics observer.
type Observer interface {
	With(lvs ...string) Observer
	Observe(float64)
}
```

## middleware

Middleware interface model definition
```go
package main

import (
	"context"
	"fmt"
	"github.com/songzhibin97/gkit/middleware"
	)

func annotate(s string) middleware.MiddleWare {
	return func(next middleware.Endpoint) middleware.Endpoint {
		return func(ctx context.Context, request interface{}) (interface{}, error) {
			fmt.Println(s, "pre")
			defer fmt.Println(s, "post")
			return next(ctx, request)
		}
	}
}

func myEndpoint(context.Context, interface{}) (interface{}, error) {
	fmt.Println("my endpoint!")
	return struct{}{}, nil
}

var (
	ctx = context.Background()
	req = struct{}{}
)

func main() {
    e := middleware.Chain(
		annotate("first"),
		annotate("second"),
		annotate("third"),
	)(myEndpoint)

	if _, err := e(ctx, req); err != nil {
		panic(err)
	}
	// Output.
	// first pre
	// second pre
	// third pre
	// my endpoint!
	// third post
	// second post
	// first post
}
```

## net

Network related encapsulation

### tcp
```go
    // Send data to the other side, with a retry mechanism
    Send(data []byte, retry *Retry) error

    // Accept data
    // length == 0 Read from Conn once and return immediately
    // length < 0 Receive all data from Conn and return it until there is no more data
    // length > 0 Receive the corresponding data from Conn and return it
    Recv(length int, retry *Retry) ([]byte, error) 

    // Read a line of '\n'
    RecvLine(retry *Retry) ([]byte, error) 

    // Read a link that has timed out
    RecvWithTimeout(length int, timeout time.Duration, retry *Retry) ([]byte, error) 

    // Write data to a link that has timed out
    SendWithTimeout(data []byte, timeout time.Duration, retry *Retry) error

    // Write data and read back
    SendRecv(data []byte, length int, retry *Retry) ([]byte, error)

    // Write data to and read from a link that has timed out
    SendRecvWithTimeout(data []byte, timeout time.Duration, length int, retry *Retry) ([]byte, error)
```

## options

Options mode interface


## overload
**general use**

```go
package main

import (
	"context"
	"github.com/songzhibin97/gkit/overload"
	"github.com/songzhibin97/gkit/overload/bbr"
)

func main() {
	// Common use
	
	// Create the Group first
	group := bbr.NewGroup()
	// If it doesn't exist, it will be created
	limiter := group.Get("key")
	f, err := limiter.Allow(context.TODO())
	if err != nil {
		// means it is overloaded and the service is not allowed to access it
		return
	}
	// Op: the actual operation type of the traffic write-back record indicator
	f(overload.DoneInfo{Op: overload.Success})
}
```

**Middleware application**

```go
package main

import (
	"context"
	"github.com/songzhibin97/gkit/overload"
	"github.com/songzhibin97/gkit/overload/bbr"
)

func main() {
	// Common use

	// Create the Group first
	group := bbr.NewGroup()
	// If it doesn't exist, it will be created
	limiter := group.Get("key")
	f, err := limiter.Allow(context.TODO())
	if err != nil {
		// means it is overloaded and the service is not allowed to access it
		return
	}
	// Op: the actual operation type of the traffic write-back record indicator
	f(overload.DoneInfo{Op: overload.Success})

	// Create Group middleware
	middle := bbr.NewLimiter()

	// in the middleware 
	// The ctx carries these two configurable valid data
	// You can get the limiter type via the ctx.Set

	// Configure to get the limiter type, you can get different limiter according to different api
	ctx := context.WithValue(context.TODO(), bbr.LimitKey, "key")

	// Configurable to report success or not
	// must be of type overload.
	ctx = context.WithValue(ctx, bbr.LimitOp, overload.Success)

	_ = middle
}
```

## parser

Provides `.go` file to `.pb` and `.pb` to `.go` 
`.go` files to `.pb` are more feature-rich, for example, providing spot staking code injection and de-duplication recognition

```go
package main

import (
	"fmt"
	"github.com/songzhibin97/gkit/parse/parseGo"
	"github.com/songzhibin97/gkit/parse/parsePb"
)

func main() {
	pgo, err := parseGo.ParseGo("gkit/parse/demo/demo.api")
	if err != nil {
		panic(err)
	}
	r := pgo.(*parseGo.GoParsePB)
	for _, note := range r.Note {
		fmt.Println(note.Text, note.Pos(), note.End())
	}
	// Output the string, if you need to import the file yourself
	fmt.Println(r.Generate())

	// Pile injection
	_ = r.PileDriving("", "start", "end", "var _ = 1")
    
	// Dismounting
	_ = r.PileDismantle("var _ = 1")
	
	ppb, err := parsePb.ParsePb("GKit/parse/demo/test.proto")
	if err != nil {
		panic(err)
	}
	// Output the string, if you need to import the file yourself
	fmt.Println(ppb.Generate())
}
```

## registry

Provides a generic interface for registering discovery, using a generic interface for external dependencies

```go
package main

// Registrar : Registrar abstraction
type Registrar interface {
	// Register : Register
	Register(ctx context.Context, service *ServiceInstance) error
	// Deregister : Logout
	Deregister(ctx context.Context, service *ServiceInstance) error
}

// Discovery : Service discovery abstraction
type Discovery interface {
	// GetService : Returns the service instance associated with the service name
	GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	// Watch : Creates a watch based on the service name
	Watch(ctx context.Context, serviceName string) (Watcher, error)
}

// Watcher : Service monitoring
type Watcher interface {
	// Next Watch requires the following conditions to be met
	// 1. the first GetService list is not empty
	// 2. Any service instance changes are found
	// If the above two conditions are not met, Next will wait indefinitely until the context is closed
	Next() ([]*ServiceInstance, error)
	// Stop : stops the monitoring behaviour
	Stop() error
}
```

## restrictor

flow limiter

### rate

leakage bucket

```go
package main

import (
	"context"
	rate2 "github.com/songzhibin97/gkit/restrictor/rate"
	"golang.org/x/time/rate"
	"time"
)

func main() {
	// The first argument is r Limit, which represents how many tokens can be generated into the Token bucket per second; Limit is actually an alias for float64
	// The second argument is b int. b represents the size of the Token bucket.
	// limit := Every(100 * time.Millisecond).
	// limiter := rate.NewLimiter(limit, 4)
	// The above means putting a Token in the bucket every 100ms, which essentially means generating 10 a second.

	// rate: golang.org/x/time/rate
	limiter := rate.NewLimiter(2, 4)

	af, wf := rate2.NewRate(limiter)

	// af.Allow() bool: take 1 token by default
	// af.Allow() == af.AllowN(time.Now(), 1)
	af.Allow()

	// af.AllowN(ctx,n) bool: can take N tokens
	af.AllowN(time.Now(), 5)

	// wf.Wait(ctx) err: wait for ctx to time out, default takes 1 token
	// wf.Wait(ctx) == wf.WaitN(ctx, 1) 
	_ = wf.Wait(context.TODO())

	// wf.WaitN(ctx, n) err: wait for ctx to time out, can take N tokens
	_ = wf.WaitN(context.TODO(), 5)
}
```

### ratelimite

Token bucket

```go
package main

import (
	"context"
	"github.com/juju/ratelimit"
	ratelimit2 "github.com/songzhibin97/gkit/restrictor/ratelimite"
	"time"
)

func main() {
	// ratelimit:github.com/juju/ratelimit
	bucket := ratelimit.NewBucket(time.Second/2, 4)

	af, wf := ratelimit2.NewRateLimit(bucket)
	// af.Allow() bool: takes 1 token by default
	// af.Allow() == af.AllowN(time.Now(), 1)
	af.Allow()

	// af.AllowN(ctx,n) bool: can take N tokens
	af.AllowN(time.Now(), 5)

	// wf.Wait(ctx) err: wait for ctx to time out, default takes 1 token
	// wf.Wait(ctx) == wf.WaitN(ctx, 1) 
	_ = wf.Wait(context.TODO())

	// wf.WaitN(ctx, n) err: wait for ctx to time out, can take N tokens
	_ = wf.WaitN(context.TODO(), 5)
}
```

## structure (common data structures)

### hashset

```go
package main

func main() {
	l := hashset.NewInt()

	for _, v := range []int{10, 12, 15} {
		l.Add(v)
	}

	if l.Contains(10) {
		fmt.Println("hashset contains 10")
	}

	l.Range(func(value int) bool {
		fmt.Println("hashset range found ", value)
		return true
	})

	l.Remove(15)
	fmt.Printf("hashset contains %d items\r\n", l.Len())
}
```


### lscq

```go
package main

func main() {
    l := lscq.NewUint64()
	
	ok := l.Enqueue(1)
	if !ok {
		panic("enqueue failed")
    }   
	v, err := l.Dequeue()
	if err != nil {
		panic("dequeue failed")
    }
	fmt.Println("lscq dequeue value:", v)
}
```


### skipmap 

```go
package main

func main() {
     m := skipmap.NewInt()

	// Correctness.
	m.Store(123, "123")
	m.Load(123)
	m.Delete(123)
	m.LoadOrStore(123)
	m.LoadAndDelete(123)
}
```

### skipset 

```go
package main
func Example() {
	l := skipset.NewInt()

	for _, v := range []int{10, 12, 15} {
		if l.Add(v) {
			fmt.Println("skipset add", v)
		}
	}

	if l.Contains(10) {
		fmt.Println("skipset contains 10")
	}

	l.Range(func(value int) bool {
		fmt.Println("skipset range found ", value)
		return true
	})

	l.Remove(15)
	fmt.Printf("skipset contains %d items\r\n", l.Len())
}
```


### zset View corresponding readme 

## sys
### mutex
    Lock-related wrappers (implements trylock, reentrant locks, etc., reentrant token locks, can also get lock indicator data)
```go
package main

func main() {
     // Get the lock
    lk := mutex.NewMutex()
    // Try to get a lock
    if lk.TryLock() {
    	// Get the lock
    	defer lk.Unlock()
    }
    // Failed to execute other logic
    
    lk.Count() // get the number of locks waiting
    
    lk.IsLocked() // whether the lock is held
    
    lk.IsWoken() // whether a waiter has been woken up internally
    
    lk.IsStarving() // whether it is in starvation mode
    
    // Reentrant locks
    // can be acquired multiple times in the same goroutine
    rvlk := mutex.NewRecursiveMutex() 
    rvlk.Lock()
    defer rvlk.Unlock()
    
    // token reentrant lock
    // Pass in the same token to enable the reentrant function
    tklk := mutex.NewTokenRecursiveMutex()
    tklk.Lock(token)
    defer tklk.Unlock(token)
}
```


## ternary
```go
package main

func main() {
    ternary.ReturnInt(true, 1, 2) 
}
```

## timeout

Timeout control between services (and a structure to handle the time format)

```go
package main

import (
	"context"
	"github.com/songzhibin97/gkit/timeout"
	"time"
)

func main() {
	// The timeout.Shrink method provides link-wide timeout control
	// Just pass in a ctx of the parent node and the timeout to be set, and he will check for you if the ctx has been set before.
	// If the timeout has been set, it will be compared with the timeout you currently set, and the smallest one will be set to ensure that the link timeout is not affected downstream.
	// d: represents the remaining timeout period
	// nCtx: the new context object
	// cancel: returns a cancel() method if the timeout was actually set successfully, or an invalid cancel if it was not, but don't worry, it can still be called normally
	d, nCtx, cancel := timeout.Shrink(context.Background(), 5*time.Second)
	// d as needed 
	// Generally determine the downstream timeout for the service, if d is too small you can just drop it
	select {
	case <-nCtx.Done():
		cancel()
	default:
		// ...
	}
	_ = d
}
```
Other
```go
    // timeout.DbJSON // provides some functionality in db json format
	// timeout.DTime // provides some functionality in db 15:04:05 format
	// DateStruct // provides some functionality in db 15:04:05 format Embedded in struct mode
	// Date // provides some functionality in db 2006-01-02 format
	// DateTime // provides some functions in db 2006-01-02 15:04:05 format
	// DateTimeStruct // provides some functions in db 2006-01-02 15:04:05 format Embed mode as struct
	// timeout.Stamp // provides some functionality in db timestamp format
```

```go
package main

import (
	"github.com/songzhibin97/gkit/timeout"
	"fmt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"time"
)

type GoStruct struct {
	DateTime timeout.DateTime
	DTime    timeout.DTime
	Date     timeout.Date
}

func main() {
	// refer to https://github.com/go-sql-driver/mysql#dsn-data-source-name for details
	dsn := "user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	db.AutoMigrate(&GoStruct{})
	db.Create(&GoStruct{
		DateTime: timeout.DateTime(time.Now()).
		DTime: timeout.DTime(time.Now()).
		Date: timeout.Date(time.Now()).
	})
	v := &GoStruct{}
	db.Find(v) // successfully found
}

```



## tools

### bind
```go

package main

// Provide an all-in-one bind tool for gin
import (
	"github.com/songzhibin97/gkit/tools/bind"

	"github.com/gin-gonic/gin"
)

type Test struct {
	Json string `json: "json" form: "json,default=jjjson"`
	Query string `json: "query" form: "query"`
}

func main() {
	r := gin.Default()
	r.POST("test", func(c * gin.Context) {
		t := Test{}
		// url : 127.0.0.1:8080/test?query=query
		// {
		// "json": "json".
		// "query": "query"
		// }
		// err := c.ShouldBindWith(&t, bind.CreateBindAll(c.ContentType()), bind.)
		// Custom binding object
		// err := c.ShouldBindWith(&t, bind.CreateBindAll(c.ContentType(), bind.SetSelectorParse([]bind.Binding{})))
		if err != nil {
			c.JSON(200, err)
			return
		}
		c.JSON(200, t)
	})
	r.Run(":8080")
}
```

### votodo
```go
package main

import "github.com/songzhibin97/gkit/tools/vto"

type CP struct {
	Z1 int `default: "1"`
	Z2 string `default: "z2"`
}

func main() {
	c1 := CP{
		Z1: 22,
		Z2: "33",
	}
	c2 := CP{}
	c3 := CP{}
	_ = vto.VoToDo(&c2,&c1)
	// c2 CP{ Z1: 22, Z2: "33"}
	// same name same type execution copy
	// must be dst, src must pass pointer type
	
	// v1.1.2 New default tag
	_ = vto.VoToDo(&c2,&c3)
	// c2 CP{ Z1: 1, Z2: "z2"}
	// same name same type execution copy
	// must dst, src must pass pointer type
	
}
```

### votodoPlus

Added field &tag & default value

## window

Provide indicator windows
```go
package main

import (
	"fmt"
	"github.com/songzhibin97/gkit/window"
	"time"
)
func main() {
	w := window.NewWindow()
	slice := []window.Index{
		{Name: "1", Score: 1}, {Name: "2", Score: 2}.
		{Name: "2", Score: 2}, {Name: "3", Score: 3}.
		{Name: "2", Score: 2}, {Name: "3", Score: 3}.
		{Name: "4", Score: 4}, {Name: "3", Score: 3}.
		{Name: "5", Score: 5}, {Name: "2", Score: 2}.
		{Name: "6", Score: 6}, {Name: "5", Score: 5}.
	}
	/*
			[{1 1} {2 2}]
		    [{2 4} {3 3} {1 1}]
		    [{1 1} {2 6} {3 6}]
		    [{3 9} {4 4} {1 1} {2 6}]
		    [{1 1} {2 8} {3 9} {4 4} {5 5}]
		    [{5 10} {3 9} {2 6} {4 4} {6 6}]
	*/
	for i := 0; i < len(slice); i += 2 {
		w.AddIndex(slice[i].Name, slice[i].Score)
		w.AddIndex(slice[i+1].Name, slice[i+1].Score)
		time.Sleep(time.Second)
		fmt.Println(w.Show())
	}
}
```

## trace
Link tracing

```go
package main

import (
	"context"
	"fmt"

	gtrace "github.com/songzhibin97/gkit/trace"
	"go.opentelemetry.io/otel/trace"
)

type _Transport struct {
}

func (tr *_Transport) Get(key string) string {
	panic("implement me")
}

func (tr *_Transport) Set(key string, value string) {
	panic("implement me")
}

func (tr *_Transport) Keys() []string {
	panic("implement me")
}
func main() {
	// trace.WithServer() server side using middleware
	// trace.WithClient() client using middleware
	tracer := gtrace.NewTracer(trace.SpanKindServer)
	ctx, span := tracer.Start(context.Background(), "Using gkit", &_Transport{})
	fmt.Println(span)
	defer tracer.End(ctx, span, "replay", nil)
}
```

## watching

System monitoring (includes cpu, mum, gc, goroutine, etc. window monitoring, dump pprof after preset fluctuation values)
