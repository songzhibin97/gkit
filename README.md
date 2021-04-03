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
致力于提供微服务以及单体服务的可用性基础组件工具集合,借鉴了一些优秀的开源项目例如:`kratos`、`go-kit`、`mosn`、`sentinel`、`gf`...
希望大家多多支持

# 目录结构
```shell
├── cache (构建缓存相关组件)
├── container (容器化组件,提供group、pool、queue)
├── downgrade (熔断降级相关组件)
├── egroup (errgroup,控制组件生命周期)
├── errors (grpc error处理)
├── generator (发号器,snowflake)
├── goroutine (提供goroutine池,控制goroutine数量激增)
├── internal (core)
├── metrics (指标接口化)
├── log (接口化日志,使用日志组件接入)
├── middleware (中间件接口模型定义)
├── net (网络相关封装)
├── options (选项模式接口化)
├── overload (服务器自适应保护,提供bbr接口,监控部署服务器状态选择流量放行,保护服务器可用性)
├── parse (文件解析,proto<->go相互解析)
├── registry (服务发现接口化)
├── restrictor (限流,提供令牌桶和漏桶接口封装)
├── timeout (超时控制,全链路保护)
└── window (滑动窗口,支持多数据类型指标窗口收集)

```

# 组件使用介绍
## cache

缓存相关组件

### singleflight

归并回源
```go
// getResources: 一般用于去数据库去获取数据
func getResources() (interface{}, error) {
	return "test", nil
}

// cache: 填充到 缓存中的数据
func cache(v interface{}) {
	return
}

// ExampleNewSingleFlight:
func ExampleNewSingleFlight() {
	singleFlight := NewSingleFlight()

	// 如果在key相同的情况下, 同一时间只有一个 func 可以去执行,其他的等待
	// 多用于缓存失效后,构造缓存,缓解服务器压力

	// 同步:
	v, err, _ := singleFlight.Do("test1", func() (interface{}, error) {
		// todo 这里去获取资源
		return getResources()
	})
	if err != nil {
		// todo 处理错误
	}
	// v 就是获取到的资源
	cache(v)

	// 异步:
	ch := singleFlight.DoChan("test2", func() (interface{}, error) {
		// todo 这里去获取资源
		return getResources()
	})

	result := <-ch
	if result.Err != nil {
		// todo 处理错误
	}
	cache(result.Val)

	// 尽力取消
	singleFlight.Forget("test2")
}

```


### buffer pool
```go
// Byte 复用
func ExampleGetBytes() {
	// size 2^6 - 2^18
	// 返回向上取整的 2的整数倍 cap, len == size
	// 其他特殊的或者在运行期间扩容的 将会被清空
	slice := GetBytes(1024)
	_ = slice
}

func ExamplePutBytes() {
	slice := make([]byte, 1024)
	// 将slice回收
	PutBytes(&slice)
}

// IOByte 复用

func ExampleGetIoPool() {
	// 创建一个缓冲区为 cap大小的 io对象
	io := GetIoPool(1024)
	_ = io
}

func ExamplePutIoPool() {
	mockIoPool := newIoBuffer(1024)
	err := PutIoPool(mockIoPool)
	if err != nil {
		// 如果一个对象已经被回收了,再次引用被回收的对象会触发错误
	}
}
```

## container

容器化组件

### group

懒加载容器
```go
func createResources() interface{} {
	return map[int]int{}
}

func createResources2() interface{} {
	return []int{}
}

var group LazyLoadGroup

func ExampleNewGroup() {
	// 类似 sync.Pool 一样
	// 初始化一个group
	group = NewGroup(createResources)
}

func ExampleGroup_Get() {
	// 如果key 不存在 调用 NewGroup 传入的 function 创建资源
	// 如果存在则返回创建的资源信息
	v := group.Get("test")
	_ = v
}

func ExampleGroup_ReSet() {
	// ReSet 重置初始化函数,同时会对缓存的 key进行清空
	group.ReSet(createResources2)
}

func ExampleGroup_Clear() {
	// 清空缓存的 buffer
	group.Clear()
}
```

### pool

类似资源池
```go
var pool Pool

type mock map[string]string

func (m *mock) Shutdown() error {
	return nil
}

// getResources: 获取资源,返回的资源对象需要实现 IShutdown 接口,用于资源回收
func getResources(c context.Context) (IShutdown, error) {
	return &mock{}, nil
}

func ExampleNewList() {
	// NewList(options ...)
	// 默认配置
	//pool = NewList()

	// 可供选择配置选项

	// 设置 Pool 连接数, 如果 == 0 则无限制
	//SetActive(100)

	// 设置最大空闲连接数
	//SetIdle(20)

	// 设置空闲等待时间
	//SetIdleTimeout(time.Second)

	// 设置期望等待
	//SetWait(false,time.Second)

	// 自定义配置
	pool = NewList(
		SetActive(100),
		SetIdle(20),
		SetIdleTimeout(time.Second),
		SetWait(false, time.Second))

	// New需要实例化,否则在 pool.Get() 会无法获取到资源
	pool.New(getResources)
}

func ExampleList_Get() {
	v, err := pool.Get(context.TODO())
	if err != nil {
		// 处理错误
	}
	// v 获取到的资源
	_ = v
}

func ExampleList_Put() {
	v, err := pool.Get(context.TODO())
	if err != nil {
		// 处理错误
	}

	// Put: 资源回收
	// forceClose: true 内部帮你调用 Shutdown回收, 否则判断是否是可回收,挂载到list上
	err = pool.Put(context.TODO(), v, false)
	if err != nil {
	  // 处理错误
	}
}


func ExampleList_Shutdown() {

	// Shutdown 回收资源,关闭所有资源
	_ = pool.Shutdown()
}
```


## downgrade

熔断降级

```go
// 与 github.com/afex/hystrix-go/hystrix 使用方法一致,只是做了抽象封装,避免因为升级对服务造成影响

var fuse Fuse

// type runFunc = func() error
// type fallbackFunc = func(error) error
// type runFuncC = func(context.Context) error
// type fallbackFuncC = func(context.Context, error) error

func mockRunFunc() runFunc {
	return func() error {
		return nil
	}
}

func mockFallbackFunc() fallbackFunc {
	return func(err error) error {
		return nil
	}
}

func mockRunFuncC() runFuncC {
	return func(ctx context.Context) error {
		return nil
	}
}

func mockFallbackFuncC() fallbackFuncC {
	return func(ctx context.Context, err error) error {
		return nil
	}
}

func ExampleNewFuse() {
	// 拿到一个熔断器
	fuse = NewFuse()
}

func ExampleHystrix_ConfigureCommand() {
	// 不设置 ConfigureCommand 走默认配置
	// hystrix.CommandConfig{} 设置参数
	fuse.ConfigureCommand("test", hystrix.CommandConfig{})
}

func ExampleHystrix_Do() {
	// Do: 同步执行 func() error, 没有超时控制 直到等到返回,
	// 如果返回 error != nil 则触发 fallbackFunc 进行降级
	err := fuse.Do("do", mockRunFunc(), mockFallbackFunc())
	if err != nil {
		// 处理 error
	}
}

func ExampleHystrix_Go() {
	// Go: 异步执行 返回 channel
	ch := fuse.Go("go", mockRunFunc(), mockFallbackFunc())
	if err := <-ch; err != nil {
		// 处理 error
	}
}

func ExampleHystrix_GoC() {
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
var admin *LifeAdmin

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

func ExampleNewLifeAdmin() {
	// 默认配置
	//admin = NewLifeAdmin()

	// 可供选择配置选项

	// 设置启动超时时间
	// <=0 不启动超时时间,注意要在shutdown处理关闭通知
	//SetStartTimeout(time.Second)

	//  设置关闭超时时间
	//	<=0 不启动超时时间
	//SetStopTimeout(time.Second)

	// 设置信号集合,和处理信号的函数
	//SetSignal(func(lifeAdmin *LifeAdmin, signal os.Signal) {
	//	return
	//}, signal...)

	admin = NewLifeAdmin(SetStartTimeout(time.Second), SetStopTimeout(time.Second), SetSignal(func(a *LifeAdmin, signal os.Signal) {
		switch signal {
		case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
			a.shutdown()
		default:
		}
	}))
}

func ExampleLifeAdmin_Add() {
	// 通过struct添加
	admin.Add(Member{
		Start:    mockStart(),
		Shutdown: mockShutdown(),
	})
}

func ExampleLifeAdmin_AddMember() {
	// 根据接口适配添加
	admin.AddMember(&mockLifeAdminer{})
}

func ExampleLifeAdmin_Start() {
	defer admin.Shutdown()
	if err := admin.Start(); err != nil {
		// 处理错误
		// 正常启动会hold主
	}
}

// 完整demo
var _admin = egroup.NewLifeAdmin()

srv := &http.Server{
		Addr: ":8080",
}
// 增加任务
_admin.Add(egroup.Member{
    Start: func(ctx context.Context) error {
        t.Log("http start")
        return goroutine.Delegate(ctx, -1, func(ctx context.Context) error {
            return srv.ListenAndServe()
        })
    },
    Shutdown: func(ctx context.Context) error {
        t.Log("http shutdown")
        return srv.Shutdown(context.Background())
    },
})
// _admin.Start() 启动
fmt.Println("error", _admin.Start())
defer _admin.shutdown()
```


## errors

封装一些error处理

## generator

发号器

### snowflake

雪花算法

```go
func ExampleNewSnowflake() {
	// 生成对象
	ids := NewSnowflake(time.Now(), 1)
	nid, err := ids.NextID()
	if err != nil {
		// 处理错误   	    
	}
	_ = nid
}
```

## goroutine

池化,控制野生goroutine

```go
var gGroup GGroup

func mockFunc() func() {
	return func() {

	}
}

func ExampleNewGoroutine() {
	// 默认配置
	//gGroup = NewGoroutine(context.TODO())

	// 可供选择配置选项

	// 设置停止超时时间
	//SetStopTimeout(time.Second)

	// 设置日志对象
	//SetLogger(&testLogger{})

	// 设置pool最大容量
	//SetMax(100)

	gGroup = NewGoroutine(context.TODO(),
		SetStopTimeout(time.Second),
		SetLogger(&testLogger{}),
		SetMax(100),
	)
}

func ExampleGoroutine_AddTask() {
	if !gGroup.AddTask(mockFunc()) {
		// 添加任务失败
	}
}

func ExampleGoroutine_AddTaskN() {
	// 带有超时控制添加任务
	if !gGroup.AddTaskN(context.TODO(), mockFunc()) {
		// 添加任务失败
	}
}

func ExampleGoroutine_ChangeMax() {
	// 修改 pool最大容量
	gGroup.ChangeMax(1000)
}

func ExampleGoroutine_Shutdown() {
	// 回收资源
	_ = gGroup.Shutdown()
}
```

## log

日志相关

```go
type testLogger struct {
	*testing.T
}

func (t *testLogger) Print(kv ...interface{}) {
	t.Log(kv...)
}

log := NewHelper(&testLogger{t}, LevelDebug)
log.Debug("debug", "v")
log.Debugf("%s,%s", "debugf", "v")
log.Info("Info", "v")
log.Infof("%s,%s", "infof", "v")
log.Warn("Warn", "v")
log.Warnf("%s,%s", "warnf", "v")
log.Error("Error", "v")
log.Errorf("%s,%s", "errorf", "v")
```

## metrics

提供指标接口,用于实现监控配置

## middleware

中间件接口模型定义

## overload

过载保护

**普通使用**

```go
// 先建立Group
group := bbr.NewGroup()
// 如果没有就会创建
limiter := group.Get("key")
f, err := limiter.Allow(ctx)
if err != nil {
// 代表已经过载了,服务不允许接入
return
}
// Op:流量实际的操作类型回写记录指标
f(overload.DoneInfo{Op: overload.Success})
```

**中间件套用**

```go
func ExampleNewGroup() {
	group := NewGroup()
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

func ExampleNewLimiter() {
	// 建立Group 中间件
	middle := NewLimiter()

	// 在middleware中 
	// ctx中携带这两个可配置的有效数据
	// 可以通过 ctx.Set

	// 配置获取限制器类型,可以根据不同api获取不同的限制器
	ctx := context.WithValue(context.TODO(), LimitKey, "key")

	// 可配置成功是否上报
	// 必须是 overload.Op 类型
	ctx = context.WithValue(ctx, LimitOp, overload.Success)

	_ = middle
}
```

## registry

提供注册发现通用接口,使用通用接口外挂依赖

## restrictor

限流器

### rate

漏桶

```go
func ExampleNewRate() {
	// 第一个参数是 r Limit。代表每秒可以向 Token 桶中产生多少 token。Limit 实际上是 float64 的别名
	// 第二个参数是 b int。b 代表 Token 桶的容量大小。
	// limit := Every(100 * time.Millisecond);
	// limiter := rate.NewLimiter(limit, 4)
	// 以上就表示每 100ms 往桶中放一个 Token。本质上也就是一秒钟产生 10 个。

	// rate: golang.org/x/time/rate
	limiter := rate.NewLimiter(2, 4)

	af, wf := NewRate(limiter)

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
func ExampleNewRateLimit() {
	// ratelimit:github.com/juju/ratelimit
	bucket := ratelimit.NewBucket(time.Second/2, 4)

	af, wf := NewRateLimit(bucket)
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
## timeout

各个服务间的超时控制

```go
func ExampleShrink() {
	// timeout.Shrink 方法提供全链路的超时控制
	// 只需要传入一个父节点的ctx 和需要设置的超时时间,他会帮你确认这个ctx是否之前设置过超时时间,
	// 如果设置过超时时间的话会和你当前设置的超时时间进行比较,选择一个最小的进行设置,保证链路超时时间不会被下游影响
	// d: 代表剩余的超时时间
	// nCtx: 新的context对象
	// cancel: 如果是成功真正设置了超时时间会返回一个cancel()方法,未设置成功会返回一个无效的cancel,不过别担心,还是可以正常调用的
	d, nCtx, cancel := Shrink(context.Background(), 5*time.Second)
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

## window

提供指标窗口
```go
func ExampleInitWindow() {
	// 初始化窗口
	w := InitWindow()

	// 增加指标
	// key:权重
	w.AddIndex("key", 1)

	// Show: 返回当前指标
	slice := w.Show()
	_ = slice
}
```

## parse

提供 `.go`文件转`.pb` 以及 `.pb`转`.go`
`.go`文件转`.pb` 功能更为丰富,例如提供定点打桩代码注入以及去重识别
```go
func ExampleGoToPB() {
	rr, err := ParseGo("GKit/parse/demo/demo.api")
	if err != nil {
		panic(err)
	}
	r := rr.(*GoParsePB)
	for _, note := range r.Note {
		t.Log(note.Text, note.Pos(), note.End())
	}
	// 输出 字符串,如果需要自行导入文件
	fmt.Println(r.Generate())
	
	// 打桩注入
	r.PileDriving("", "start", "end", "var _ = 1")
}

func ExamplePbToGo()  {
    r, err := ParsePb("GKit/parse/demo/test.proto")
	if err != nil {
		panic(err)
	}
	// 输出 字符串,如果需要自行导入文件
	fmt.Println(r.Generate())
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