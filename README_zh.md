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


# 项目简介
致力于提供微服务以及单体服务的可用性基础组件工具集合,借鉴了一些优秀的开源项目例如:`kratos`、`go-kit`、`mosn`、`sentinel`、`gopkg`... 希望大家多多支持

# 目录结构
```shell
├── cache (构建缓存相关组件)
  ├── buffer (提供byte数组复用以及io buffer封装)
  ├── mbuffer (buffer 类似实现) 
  ├── local_cache (提供本地key-value构建本地缓存的封装实现)
  ├── singleflight (提供高并发情况下防止重复任务,一般用于cache miss后填补cache场景)
├── coding (提供对象序列化/反序列化接口化, 提供json、proto、xml、yaml 实例方法)
├── concurrent (在并发中使用channel的最佳实践)
  ├── fan_in (扇入模式,常用与生产者消费者模型中多个生产者,一个消费者)
  ├── fan_out (扇出模式,常用与生产着消费者模型中一个生产者,多个消费者)
  ├── or_done (并发场景下任意一个任务完成后立即返回)
  ├── orderly (在并发场景下也能保持有序的完成返回)
  ├── map_reduce 
  ├── stream (提供数据生产流封装,以及处理流的实现)
  ├── pipeline (并发变为串行)
├── container (容器化组件,提供group、pool、queue)
  ├── group (提供了容器懒加载模式,类似sync.Pool,在使用时使用key获取对应容器实例,如果不存在则进行生成)
  ├── pool (提供了pool的封装抽象,以及使用list对接口的实现)
  ├── queue
    ├── codel (对列实现可控制延时算法,对积压任务实现制裁)
├── delayed (延时任务-单机版)
├── distributed (分布式任务,提供了标准化接口以及redis、mysql、pgsql、mongodb对应的实现)
├── downgrade (熔断降级相关组件)
├── egroup (errgroup,控制组件生命周期)
├── errors (grpc error处理)
├── gctuner (go1.19前优化gc利器)
├── generator (发号器,snowflake)
├── goroutine (提供goroutine池,控制goroutine数量激增)
├── log (接口化日志,使用日志组件接入)
├── metrics (指标接口化)
├── middleware (中间件接口模型定义)
├── net (网络相关封装)
  ├── tcp
├── options (选项模式接口化)
├── overload (服务器自适应保护,提供bbr接口,监控部署服务器状态选择流量放行,保护服务器可用性)
  ├── bbr (自适应限流)
├── page_token (google aip next token 实现)  
├── parser (文件解析,proto<->go相互解析)
  ├── parseGo (解析go生成pb)
  ├── parsePb (解析pb生成go)
├── registry (服务发现接口化、google sre subset实现)
├── restrictor (限流,提供令牌桶和漏桶接口封装)
  ├── client_throttling (客户端节流)
  ├── rate 
  ├── ratelimite 
├── structure (常用数据结构)
  ├── hashset (哈希表)
  ├── lscq (无锁无边界队列,支持arm)
  ├── skipmap 
  ├── skipset 
  ├── zset 
├── sync
    ├── cpu (获取Linux平台下的系统信息,包括cpu主频、cpu使用率等)
    ├── fastrand (随机数)
    ├── goid (获取goroutine id)
    ├── mutex (提供trylock、重入锁和token重入锁)
    ├── nanotime (时间戳优化)
    ├── once (once 更强大的实现,设置once函数增加返回error,失败后可重试)
    ├── queue (无锁队列)
    ├── safe (底层string,slice 结构)
    ├── stringx (string 增强版)
    ├── syncx (sync 增强版)
    ├── xxhash3 
├── ternary (三元表达式)    
├── timeout (超时控制,全链路保护、提供一些数据库处理时间的封装实现)
  ├── ctime (链路超时控制)
  ├── c_json (适配数据库json类型)
  ├── d_time (适配数据库 只存储时间)
  ├── date (适配数据库 只存储日期)
  ├── date_struct (适配数据库 只存储日期)
  ├── datetime (适配数据库 存储datetime)
  ├── datetime_struct (适配数据库 存储datetime)
  ├── stamp (适配数据库 存储时间戳)
  ├── human (提供可视化时间间距)
├── tools 
  ├── bind (绑定工具,常用与gin框架中自定义绑定数据,例如同时绑定query和json)
  ├── deepcopy (深拷贝)
  ├── float (浮点数截断工具)
  ├── match (基础匹配器,根据通配符匹配)
  ├── pointer (指针工具)
  ├── pretty (格式化json)
  ├── reflect2value (基础字段映射)
  ├── rand_string (随机字符串)
  ├── vto (具有相同类型的函数赋值,解放双手,通常用于vo->do对象转换)
    ├── vtoPlus (新增plus 支持字段,tag以及默认值绑定)
├── trace (链路追踪)
├── watching (监控cpu、mum、gc、goroutine等指标信息,在波动的情况下自动dump pprof指标)
└── window (滑动窗口,支持多数据类型指标窗口收集)

```

# 下载使用
```shell
# go get github.com/songzhibin97/gkit@master
go get github.com/songzhibin97/gkit
```

# 组件使用介绍
## cache

缓存相关组件
> buffer&mbuffer 提供的功能类似,buffer多了一些封装,以及实现了io方面的一些接口,而mbuffer仅仅是一个memory的缓存;在生命周期较短且频繁的情况下更适用;
> local_cache 提供了本地的数据缓存,也有一些失效机制,可以设置过期时间,以及定时清理过期数据,但是他现在比较旧了,如果需要的话有泛型版本 https://github.com/songzhibin97/go-baseutils/blob/main/app/bcache
> singleflight 封装了 golang.org/x/sync/singleflight,防止变更带来的影响.

### buffer pool
```go
package main

import (
	"fmt"
	"github.com/songzhibin97/gkit/cache/buffer"
)

func main() {
	// Byte复用

	// size 2^6 - 2^18
	// 返回向上取整的 2的整数倍 cap, len == size
	// 其他特殊的或者在运行期间扩容的 将会被清空
	slice := buffer.GetBytes(1024)
	fmt.Println(len(*slice), cap(*slice)) // 1024 1024

	// 回收
	// 注意: 回收以后不可在引用
	buffer.PutBytes(slice)

	// IOByte 复用

	// io buffer.IoBuffer interface
	io := buffer.GetIoPool(1024)

	// 如果一个对象已经被回收了,再次引用被回收的对象会触发错误
	err := buffer.PutIoPool(io)
	if err != nil {
		// 处理错误 	    
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
	// 默认配置
	//ch = NewCache()

	// 可供选择的配置选项

	// 设置间隔时间
	// SetInternal(interval time.Duration)

	// 设置默认的超时时间
	// SetDefaultExpire(expire time.Duration)

	// 设置周期的执行函数,默认(不设置)是扫描全局清除过期的k
	// SetFn(fn func())

	// 设置触发删除后的捕获函数, 数据删除后回调用设置的捕获函数
	// SetCapture(capture func(k string, v interface{}))

	// 设置初始化存储的成员对象
	// SetMember(m map[string]Iterator)

	ch = NewCache(SetInternal(1000),
		SetDefaultExpire(10000),
		SetCapture(func(k string, v interface{}) {
			log.Println(k, v)
		}))
}

func ExampleCacheStorage() {
	// Set 添加cache 无论是否存在都会覆盖
	ch.Set("k1", "v1", DefaultExpire)

	// SetDefault 无论是否存在都会覆盖
	// 偏函数模式,默认传入超时时间为创建cache的默认时间
	ch.SetDefault("k1", 1)

	// SetNoExpire
	// 偏函数模式,默认传入超时时间为永不过期
	ch.SetNoExpire("k1", 1.1)

	// Add 添加cache 如果存在的话会抛出异常
	err := ch.Add("k1", nil, DefaultExpire)
	CacheErrExist(err) // true

	// Replace 如果有就设置没有就抛出错误
	err = ch.Replace("k2", make(chan struct{}), DefaultExpire)
	CacheErrNoExist(err) // true
}

func ExampleGet() {
	// Get 根据key获取 cache 保证有效期内的kv被取出
	v, ok := ch.Get("k1")
	if !ok {
		// v == nil
	}
	_ = v

	// GetWithExpire 根据key获取 cache 并带出超时时间
	v, t, ok := ch.GetWithExpire("k1")
	if !ok {
		// v == nil
	}
	// 如果超时时间是 NoExpire t.IsZero() == true
	if t.IsZero() {
		// 没有设置超时时间
	}

	// Iterator 返回 cache 中所有有效的对象
	mp := ch.Iterator()
	for s, iterator := range mp {
		log.Println(s, iterator)
	}
	
	// Count 返回member数量
	log.Println(ch.Count())
}

func ExampleIncrement() {
	ch.Set("k3", 1, DefaultExpire)
	ch.Set("k4", 1.1, DefaultExpire)
	// Increment 为k对应的value增加n n必须为数字类型
	err := ch.Increment("k3", 1)
	if CacheErrExpire(err) || CacheErrExist(CacheTypeErr) {
		// 未设置成功
	}
	_ = ch.IncrementFloat("k4", 1.1)

	// 如果你知道设置的k的具体类型 还可以使用类型确定的 increment函数
	// ch.IncrementInt(k string, v int)
	// ...
	// ch.IncrementFloat32(k string, v flot32)
	// ...

	// Decrement 同理
}

func ExampleDelete() {
	// Delete 如果设置了 capture 会触发不或函数
	ch.Delete("k1")

	// DeleteExpire 删除所有过期了的key, 默认的 capture 就是执行 DeleteExpire()
	ch.DeleteExpire()
}

func ExampleChangeCapture() {
	// 提供了在运行中改变捕获函数的方法
	// ChangeCapture
	ch.ChangeCapture(func(k string, v interface{}) {
		log.Println(k, v)
	})
}

func ExampleSaveLoad() {
	// 写入文件采用go独有的gob协议

	io := buffer.NewIoBuffer(1000)

	// Save 传入一个 w io.Writer 参数 将 cache中的 member 成员写入w中
	_ = ch.Save(io)

	// SaveFile 传入path 写到文件中
	_ = ch.SaveFile("path")

	// Load 传入一个 r io.Reader对象 从 r中读取写回到 member中
	_ = ch.Load(io)

	// LoadFile 传入path 读取文件内容
	_ = ch.LoadFile("path")
}

func ExampleFlush()  {
	// Flush 释放member成员
	ch.Flush()
}

func ExampleShutdown()  {
	// Shutdown 释放对象
	ch.Shutdown()
}
```

### singleflight

归并回源
```go
package main

import (
	"github.com/songzhibin97/gkit/cache/singleflight"
)

// getResources: 一般用于去数据库去获取数据
func getResources() (interface{}, error) {
	return "test", nil
}

// cache: 填充到 缓存中的数据
func cache(v interface{}) {
	return
}

func main() {
	f := singleflight.NewSingleFlight()

	// 同步:
	v, err, _ := f.Do("test1", func() (interface{}, error) {
		// 获取资源
		return getResources()
	})
	if err != nil {
		// 处理错误
	}
	// 存储到buffer
	// v就是获取到的资源
	cache(v)

	// 异步
	ch := f.DoChan("test2", func() (interface{}, error) {
		// 获取资源
		return getResources()
	})

	// 等待获取资源完成后,会将结果通过channel返回
	result := <-ch
	if result.Err != nil {
		// 处理错误
	}
	
	// 存储到buffer
	// result.Val就是获取到的资源
	cache(result.Val)
	
	// 尽力取消
	f.Forget("test2")
}
```


## coding
> 对象序列化反序列化接口以及实例封装,只需要导入匿名,例如json `_ "github.com/songzhibin97/gkit/coding/json"` 也可以实现对应接口后,进行注册,好处是可以控制全局序列化对象以及使用库的统一

```go
package main

import (
	"fmt"
	"github.com/songzhibin97/gkit/coding"
	_ "github.com/songzhibin97/gkit/coding/json" // 一定要提前导入!!!
)

func main() {
	t := struct {
		Gkit  string
		Lever int
	}{"Gkit", 200}
	fmt.Println(coding.GetCode("json").Name())
	data, err := coding.GetCode("json").Marshal(t)
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(string(data)) // {"Gkit":"Gkit","Lever":200}
	v := struct {
		Gkit  string
		Lever int
	}{}
	coding.GetCode("json").Unmarshal(data,&v)
	fmt.Println(v) // {Gkit 200}
}
```

## concurrent

> 并发中channel最佳实践,包含 fan_in,fan_out,map_reduce,or_done,orderly,pipeline,stream,泛型版本可以参考 https://github.com/songzhibin97/go-baseutils/tree/main/app/bconcurrent


## container

> group 适用于生命周期较长对象的懒加载,类似于 sync.Pool,但是可以自定义创建函数,以及更换初始化函数
> pool 池化对象,通过配置可以设置最大连接数以及等待连接数,同步异步获取连接,以及连接的生命周期管理,可以自定义创建函数,以及更换初始化函数
> codel 实现codel算法,可以用于限流,以及熔断

### group

懒加载容器
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
	// 类似 sync.Pool 一样
	// 初始化一个group
	g := group.NewGroup(createResources)

	// 如果key 不存在 调用 NewGroup 传入的 function 创建资源
	// 如果存在则返回创建的资源信息
	v := g.Get("test")
	fmt.Println(v) // map[1:1 2:2]
	v.(map[int]int)[1] = 3
	fmt.Println(v) // map[1:3 2:2]
	v2 := g.Get("test")
	fmt.Println(v2) // map[1:3 2:2]

	// ReSet 重置初始化函数,同时会对缓存的 key进行清空
	g.ReSet(createResources2)
	v3 := g.Get("test")
	fmt.Println(v3) // []int{1,2,3}
	
	// 清空缓存的 buffer
	g.Clear()
}
```

### pool

类似资源池
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

// getResources: 获取资源,返回的资源对象需要实现 IShutdown 接口,用于资源回收
func getResources(c context.Context) (pool.IShutdown, error) {
	return &mock{"mockKey": "mockValue"}, nil
}

func main() {
	// pool.NewList(options ...)
	// 默认配置
	// p = pool.NewList()

	// 可供选择配置选项

	// 设置 Pool 连接数, 如果 == 0 则无限制
	// pool.SetActive(100)

	// 设置最大空闲连接数
	// pool.SetIdle(20)

	// 设置空闲等待时间
	// pool.SetIdleTimeout(time.Second)

	// 设置期望等待
	// pool.SetWait(false,time.Second)

	// 自定义配置
	p = pool.NewList(
		pool.SetActive(100),
		pool.SetIdle(20),
		pool.SetIdleTimeout(time.Second),
		pool.SetWait(false, time.Second))

	// New需要实例化,否则在 pool.Get() 会无法获取到资源
	p.NewQueue(getResources)

	v, err := p.Get(context.TODO())
	if err != nil {
		// 处理错误
	}
	fmt.Println(v) // &map[mockKey:mockValue]

	// Put: 资源回收
	// forceClose: true 内部帮你调用 Shutdown回收, 否则判断是否是可回收,挂载到list上
	err = p.Put(context.TODO(), v, false)
	if err != nil {
		// 处理错误  	    
	}
	
	// Shutdown 回收资源,关闭所有资源
	_ = p.Shutdown()
}
```

### queue
codel实现
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

	// start 体现 CoDel 状态信息
	start := queue.Stat()
	fmt.Println(start)

	go func() {
		// 实际消费的地方
		queue.Pop()
	}()
	if err := queue.Push(context.TODO()); err != nil {
		if err == bbr.LimitExceed {
			// todo 处理过载保护错误
		} else {
			// todo 处理其他错误
		}
	}
}
```

## delayed

延时任务(单机版)

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
	// 创建延时对象
	// delayed.SetSingle() 设置监控信号
	// delayed.SetSingleCallback() 设置信号回调
	// delayed.SetWorkerNumber() 设置工作协程
	// delayed.SetCheckTime() 设置监控时间
	n := delayed.NewDispatchingDelayed()

	// 添加延时任务
	n.AddDelayed(mockDelayed{exec: 1})
	
	// 强制刷新
	n.Refresh()
	
	// 关闭
	n.Close()
}

```

## distributed 

分布式任务(详细使用看测试用例)

## downgrade

熔断降级

```go
// 与 github.com/afex/hystrix-go 使用方法一致,只是做了抽象封装,避免因为升级对服务造成影响
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
	// 拿到一个熔断器
	fuse = downgrade.NewFuse()

	// 不设置 ConfigureCommand 走默认配置
	// hystrix.CommandConfig{} 设置参数
	fuse.ConfigureCommand("test", hystrix.CommandConfig{})

	// Do: 同步执行 func() error, 没有超时控制 直到等到返回,
	// 如果返回 error != nil 则触发 FallbackFunc 进行降级
	err := fuse.Do("do", mockRunFunc(), mockFallbackFunc())
	if err != nil {
		// 处理 error
	}

	// Go: 异步执行 返回 channel
	ch := fuse.Go("go", mockRunFunc(), mockFallbackFunc())
	select {
	case err = <-ch:
	// 处理错误
	case <-outCH:
		break
	}

	// GoC: Do/Go 实际上最终调用的就是GoC, Do主处理了异步过程
	// GoC可以传入 context 保证链路超时控制
	fuse.GoC(context.TODO(), "goc", mockRunFuncC(), mockFallbackFuncC())
}
```

## egroup

组件生命周期管理
```go
// errorGroup 
// 级联控制,如果有组件发生错误,会通知group所有组件退出
// 声明生命周期管理

package main

import (
	"context"
	"fmt"
	"github.com/songzhibin97/gkit/egroup"
	"github.com/songzhibin97/gkit/goroutine"
	"net/http"
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
	// 默认配置
	//admin = egroup.NewLifeAdmin()

	// 可供选择配置选项

	// 设置启动超时时间
	// <=0 不启动超时时间,注意要在shutdown处理关闭通知
	// egroup.SetStartTimeout(time.Second)

	//  设置关闭超时时间
	//	<=0 不启动超时时间
	// egroup.SetStopTimeout(time.Second)

	// 设置信号集合,和处理信号的函数
	// egroup.SetSignal(func(lifeAdmin *LifeAdmin, signal os.Signal) {
	//	return
	// }, signal...)

	admin = egroup.NewLifeAdmin(egroup.SetStartTimeout(time.Second), egroup.SetStopTimeout(time.Second),
		egroup.SetSignal(func(a *egroup.LifeAdmin, signal os.Signal) {
			switch signal {
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
				a.Shutdown()
			default:
			}
		}))

	// 通过struct添加
	admin.Add(egroup.Member{
		Start:    mockStart(),
		Shutdown: mockShutdown(),
	})

	// 通过接口适配
	admin.AddMember(&mockLifeAdminer{})

	// 启动
	defer admin.Shutdown()
	if err := admin.Start(); err != nil {
		// 处理错误
		// 正常启动会hold主
	}
}

func Demo() {
	// 完整demo
	var _admin = egroup.NewLifeAdmin()

	srv := &http.Server{
		Addr: ":8080",
	}
	// 增加任务
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
	// _admin.Start() 启动
	fmt.Println("error", _admin.Start())
	defer _admin.Shutdown()
}

```


## errors

封装一些error处理

```go
package main

import (
	"fmt"
	"net/http"
	"time"
	
	"github.com/songzhibin97/gkit/errors"
)
func main() {
	err := errors.Errorf(http.StatusBadRequest, "原因", "携带信息%s", "测试")
	err2 := err.AddMetadata(map[string]string{"time": time.Now().String()}) // 携带元信息
	// err 是原来的错误 err2 是带有元信息的错误
	fmt.Println(errors.Is(err,err2)) // ture
	// 可以解析err2 来获取更多的信息
	fmt.Println(err2.Metadata["time"]) // meta
}
```

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

发号器

### snowflake

雪花算法

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
		// 处理错误
	}
	fmt.Println(nid)
}

```

## goroutine

池化,控制野生goroutine

```go
package main

import (
	"context"
	"fmt"
	"github.com/songzhibin97/gkit/goroutine"
	"time"
)

var gGroup goroutine.GGroup

func mockFunc() func() {
	return func() {
		fmt.Println("ok")
	}
}

func main() {
	// 默认配置
	//gGroup = goroutine.NewGoroutine(context.TODO())

	// 可供选择配置选项

	// 设置停止超时时间
	// goroutine.SetStopTimeout(time.Second)

	// 设置日志对象
	// goroutine.SetLogger(&testLogger{})

	// 设置pool最大容量
	// goroutine.SetMax(100)

	gGroup = goroutine.NewGoroutine(context.TODO(),
		goroutine.SetStopTimeout(time.Second),
		goroutine.SetMax(100),
	)

	// 添加任务
	if !gGroup.AddTask(mockFunc()) {
		// 添加任务失败
	}

	// 带有超时控制添加任务
	if !gGroup.AddTaskN(context.TODO(), mockFunc()) {
		// 添加任务失败
	}

	// 修改 pool最大容量
	gGroup.ChangeMax(1000)

	// 回收资源
	_ = gGroup.Shutdown()
}
```

## log

日志相关

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

提供指标接口,用于实现监控配置
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

中间件接口模型定义
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

func main()  {
    e := middleware.Chain(
		annotate("first"),
		annotate("second"),
		annotate("third"),
	)(myEndpoint)

	if _, err := e(ctx, req); err != nil {
		panic(err)
	}
	// Output:
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

网络相关封装

### tcp
```go
    // 发送数据至对端,有重试机制
    Send(data []byte, retry *Retry) error

    // 接受数据
    // length == 0 从 Conn一次读取立即返回
    // length < 0 从 Conn 接收所有数据，并将其返回，直到没有数据
    // length > 0 从 Conn 接收到对应的数据返回
    Recv(length int, retry *Retry) ([]byte, error) 

    // 读取一行 '\n'
    RecvLine(retry *Retry) ([]byte, error) 

    // 读取已经超时的链接
    RecvWithTimeout(length int, timeout time.Duration, retry *Retry) ([]byte, error) 

    // 写入数据给已经超时的链接
    SendWithTimeout(data []byte, timeout time.Duration, retry *Retry) error

    // 写入数据并读取返回
    SendRecv(data []byte, length int, retry *Retry) ([]byte, error)

    // 将数据写入并读出已经超时的链接
    SendRecvWithTimeout(data []byte, timeout time.Duration, length int, retry *Retry) ([]byte, error)
```


## options

选项模式接口


## overload

过载保护

**普通使用**

```go
package main

import (
	"context"
	"github.com/songzhibin97/gkit/overload"
	"github.com/songzhibin97/gkit/overload/bbr"
)

func main() {
	// 普通使用
	
	// 先建立Group
	group := bbr.NewGroup()
	// 如果没有就会创建
	limiter := group.Get("key")
	f, err := limiter.Allow(context.TODO())
	if err != nil {
		// 代表已经过载了,服务不允许接入
		return
	}
	// Op:流量实际的操作类型回写记录指标
	f(overload.DoneInfo{Op: overload.Success})
}
```

**中间件套用**

```go
package main

import (
	"context"
	"github.com/songzhibin97/gkit/overload"
	"github.com/songzhibin97/gkit/overload/bbr"
)

func main() {
	// 普通使用

	// 先建立Group
	group := bbr.NewGroup()
	// 如果没有就会创建
	limiter := group.Get("key")
	f, err := limiter.Allow(context.TODO())
	if err != nil {
		// 代表已经过载了,服务不允许接入
		return
	}
	// Op:流量实际的操作类型回写记录指标
	f(overload.DoneInfo{Op: overload.Success})

	// 建立Group 中间件
	middle := bbr.NewLimiter()

	// 在middleware中 
	// ctx中携带这两个可配置的有效数据
	// 可以通过 ctx.Set

	// 配置获取限制器类型,可以根据不同api获取不同的限制器
	ctx := context.WithValue(context.TODO(), bbr.LimitKey, "key")

	// 可配置成功是否上报
	// 必须是 overload.Op 类型
	ctx = context.WithValue(ctx, bbr.LimitOp, overload.Success)

	_ = middle
}
```

## parser

提供 `.go`文件转`.pb` 以及 `.pb`转`.go`
`.go`文件转`.pb` 功能更为丰富,例如提供定点打桩代码注入以及去重识别
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
	// 输出 字符串,如果需要自行导入文件
	fmt.Println(r.Generate())

	// 打桩注入
	_ = r.PileDriving("", "start", "end", "var _ = 1")
    
	// 拆装
	_ = r.PileDismantle("var _ = 1")
	
	ppb, err := parsePb.ParsePb("GKit/parse/demo/test.proto")
	if err != nil {
		panic(err)
	}
	// 输出 字符串,如果需要自行导入文件
	fmt.Println(ppb.Generate())
}
```

## registry

提供注册发现通用接口,使用通用接口外挂依赖

```go
package main

// Registrar: 注册抽象
type Registrar interface {
	// Register: 注册
	Register(ctx context.Context, service *ServiceInstance) error
	// Deregister: 注销
	Deregister(ctx context.Context, service *ServiceInstance) error
}

// Discovery: 服务发现抽象
type Discovery interface {
	// GetService: 返回服务名相关的服务实例
	GetService(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
	// Watch: 根据服务名创建监控
	Watch(ctx context.Context, serviceName string) (Watcher, error)
}

// Watcher: 服务监控
type Watcher interface {
	// Watch需要满足以下条件
	// 1. 第一次 GetService 的列表不为空
	// 2. 发现任何服务实例更改
	// 不满足以上两种条件,Next则会无限等待直到上下文截止
	Next() ([]*ServiceInstance, error)
	// Stop: 停止监控行为
	Stop() error
}
```


## restrictor

限流器

### rate

漏桶

```go
package main

import (
	"context"
	rate2 "github.com/songzhibin97/gkit/restrictor/rate"
	"golang.org/x/time/rate"
	"time"
)

func main() {
	// 第一个参数是 r Limit。代表每秒可以向 Token 桶中产生多少 token。Limit 实际上是 float64 的别名
	// 第二个参数是 b int。b 代表 Token 桶的容量大小。
	// limit := Every(100 * time.Millisecond);
	// limiter := rate.NewLimiter(limit, 4)
	// 以上就表示每 100ms 往桶中放一个 Token。本质上也就是一秒钟产生 10 个。

	// rate: golang.org/x/time/rate
	limiter := rate.NewLimiter(2, 4)

	af, wf := rate2.NewRate(limiter)

	// af.Allow()bool: 默认取1个token
	// af.Allow() == af.AllowN(time.Now(), 1)
	af.Allow()

	// af.AllowN(ctx,n)bool: 可以取N个token
	af.AllowN(time.Now(), 5)

	// wf.Wait(ctx) err: 等待ctx超时,默认取1个token
	// wf.Wait(ctx) == wf.WaitN(ctx, 1) 
	_ = wf.Wait(context.TODO())

	// wf.WaitN(ctx, n) err: 等待ctx超时,可以取N个token
	_ = wf.WaitN(context.TODO(), 5)
}
```
### ratelimite

令牌桶

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
	// af.Allow()bool: 默认取1个token
	// af.Allow() == af.AllowN(time.Now(), 1)
	af.Allow()

	// af.AllowN(ctx,n)bool: 可以取N个token
	af.AllowN(time.Now(), 5)

	// wf.Wait(ctx) err: 等待ctx超时,默认取1个token
	// wf.Wait(ctx) == wf.WaitN(ctx, 1) 
	_ = wf.Wait(context.TODO())

	// wf.WaitN(ctx, n) err: 等待ctx超时,可以取N个token
	_ = wf.WaitN(context.TODO(), 5)
}
```

## structure (常用数据结构)

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

### zset 查看对应readme 

## sys
### mutex
    锁相关封装（实现了trylock、重入锁等、重入token锁,还可以获取锁指标数据）
```go
package main

func main() {
	// 获取锁
    lk := mutex.NewMutex()
    // 尝试获取锁
    if lk.TryLock() {
    	// 获取到锁
    	defer lk.Unlock()
    }
    // 获取失败执行其他逻辑
    
    lk.Count() // 获取等待锁的数量
    
    lk.IsLocked() // 锁是否被持有
    
    lk.IsWoken() // 内部是否有等待者被唤醒
    
    lk.IsStarving() // 是否处于饥饿模式
    
    // 重入锁
    // 在同一个goroutine可以多次获取
    rvlk := mutex.NewRecursiveMutex() 
    rvlk.Lock()
    defer rvlk.Unlock()
    
    // token重入锁
    // 传入相同token 可以实现重入功能
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

各个服务间的超时控制(以及处理时间格式的结构体)

```go
package main

import (
	"context"
	"github.com/songzhibin97/gkit/timeout"
	"time"
)

func main() {
	// timeout.Shrink 方法提供全链路的超时控制
	// 只需要传入一个父节点的ctx 和需要设置的超时时间,他会帮你确认这个ctx是否之前设置过超时时间,
	// 如果设置过超时时间的话会和你当前设置的超时时间进行比较,选择一个最小的进行设置,保证链路超时时间不会被下游影响
	// d: 代表剩余的超时时间
	// nCtx: 新的context对象
	// cancel: 如果是成功真正设置了超时时间会返回一个cancel()方法,未设置成功会返回一个无效的cancel,不过别担心,还是可以正常调用的
	d, nCtx, cancel := timeout.Shrink(context.Background(), 5*time.Second)
	// d 根据需要判断 
	// 一般判断该服务的下游超时时间,如果d过于小,可以直接放弃
	select {
	case <-nCtx.Done():
		cancel()
	default:
		// ...
	}
	_ = d
}
```
其他
```go
    // timeout.DbJSON // 提供db json格式的一些功能
	// timeout.DTime // 提供db 15:04:05 格式的一些功能
	// timeout.DateStruct // 提供db 15:04:05 格式的一些功能 嵌入模式为struct
	// timeout.Date // 提供db 2006-01-02 格式的一些功能
	// timeout.DateTime // 提供db 2006-01-02 15:04:05 格式的一些功能
	// timeout.DateTimeStruct // 提供db 2006-01-02 15:04:05 格式的一些功能 嵌入模式为struct
	// timeout.Stamp // 提供db 时间戳格式的一些功能
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
	// 参考 https://github.com/go-sql-driver/mysql#dsn-data-source-name 获取详情
	dsn := "user:pass@tcp(127.0.0.1:3306)/dbname?charset=utf8mb4&parseTime=True&loc=Local"
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(err)
	}
	db.AutoMigrate(&GoStruct{})
	db.Create(&GoStruct{
		DateTime: timeout.DateTime(time.Now()),
		DTime:    timeout.DTime(time.Now()),
		Date:     timeout.Date(time.Now()),
	})
	v := &GoStruct{}
	db.Find(v) // 成功查出
}

```


## tools

### bind
```go

package main

// 为 gin提供一个全能bind工具
import (
	"github.com/songzhibin97/gkit/tools/bind"

	"github.com/gin-gonic/gin"
)

type Test struct {
	Json  string `json:"json" form:"json,default=jjjson"`
	Query string `json:"query" form:"query"`
}

func main() {
	r := gin.Default()
	r.POST("test", func(c *gin.Context) {
		t := Test{}
		// url : 127.0.0.1:8080/test?query=query
		// {
		//  "json":"json",
		//  "query":"query"
		// }
		// err := c.ShouldBindWith(&t, bind.CreateBindAll(c.ContentType()),bind.)
		// 自定义binding对象
		// err := c.ShouldBindWith(&t, bind.CreateBindAll(c.ContentType(),bind.SetSelectorParse([]bind.Binding{})))
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
	Z1 int `default:"1"`
	Z2 string `default:"z2"`
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
	// 相同名称相同类型的执行复制
	// 一定要dst、src 必须传指针类型
	
	// v1.1.2 新增default标签
	_ = vto.VoToDo(&c2,&c3)
	// c2 CP{ Z1: 1, Z2: "z2"}
	// 相同名称相同类型的执行复制
	// 一定要dst、src 必须传指针类型
	
}
```

### votodoPlus

增加了字段&tag&默认值


## window

提供指标窗口
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
		{Name: "1", Score: 1}, {Name: "2", Score: 2},
		{Name: "2", Score: 2}, {Name: "3", Score: 3},
		{Name: "2", Score: 2}, {Name: "3", Score: 3},
		{Name: "4", Score: 4}, {Name: "3", Score: 3},
		{Name: "5", Score: 5}, {Name: "2", Score: 2},
		{Name: "6", Score: 6}, {Name: "5", Score: 5},
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
链路追踪

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
	// trace.WithServer() 服务端使用中间件
	// trace.WithClient() 客户端使用中间件
	tracer := gtrace.NewTracer(trace.SpanKindServer)
	ctx, span := tracer.Start(context.Background(), "使用gkit", &_Transport{})
	fmt.Println(span)
	defer tracer.End(ctx, span, "replay", nil)
}
```

## watching

系统监控(包含cpu、mum、gc、goroutine等窗口监控,在预设波动值后dump pprof)