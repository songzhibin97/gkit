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

# 目录结构
```shell
├── cache (构建缓存相关组件)
├── container (容器化组件,提供group、pool、queue)
├── downgrade (熔断降级相关组件)
├── egroup (errgroup,控制组件生命周期)
├── errors
├── goroutine (提供goroutine池,控制goroutine数量激增)
├── internal (core)
├── log (接口化日志,使用日志组件接入)
├── middleware (中间件接口模型定义)
├── overload (服务器自适应保护,提供bbr接口,监控部署服务器状态选择流量放行,保护服务器可用性)
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
// 与 golang.org/x/sync/singleflight 使用方法一致,只是做了抽象封装,避免因为升级对服务造成影响

g := singleflight.NewSingleFlight()

// 如果在key相同的情况下, 同一时间只有一个 func 可以去执行,其他的等待
// 多用于缓存失效后,构造缓存,缓解服务器压力
// shade 表示是否将 v 分配给多个请求者
v, err, shade := g.Do(key, func()(interface{}, error))
if err != nil {
	// ...
}
// 判断数据有效后,将v 放到cache
cache(v)

// 异步调用
ch := g.DoChan(key, func()(interface{}, error))
v <- ch
// v.Val
// v.Err
// v.Shared


// 尽力取消
g.Forget(key)

```

## container

容器化组件

### group

懒加载容器
```go
// 声明一个group
// 传入一个函数
 g := group.NewGroup(func() interface{})
 // 如果key 存在 则将 对应的 func执行结果返回
 // 如果不存在 在内部维护的 mapping 建立映射,下次使用时候可以快速返回
 v := g.Get(key)
 
 // 重置
 // 会将之前构造的mapping 清空置换为新的函数
 g.ReSet(func() interface{})
```

### pool

类似资源池
```go
conf := &pool.Config{
	    // Active: 池中最大数量,如果 == 0 则不进行显示
		Active:      1,
		// Idle: 最大空闲数
		Idle:        1,
		// IdleTimeout: 空闲等待的时间
		IdleTimeout: 90 * time.Second,
		// WaitTimeout: 如果已经用尽,等待连接归还的时间
		WaitTimeout: 10 * time.Millisecond,
		// Wait: 是否等待, 如果为 false WaitTimeout 不再有效
		Wait:        false,
}
// 初始化pool 
p := pool.NewList(conf)
// 设置如果需要新增资源的初始化函数
p.New(func(ctx context.Context) (IShutdown, error))

v := p.Get(ctx)

// forceClose: 是否强制关闭
_ = p.Put(ctx,v,forceClose)

// 资源回收退出
_ = p.Shutdown()
```

### queue/CoDel

对列管理算法,根据实际的消费情况,算出该请求是否需要等待还是快速失败.

```go
// 默认配置
q := codel.Default()

// 自定义配置
q := codel.New(*Config)

// Reload: 重新设置配置信息
q.Reload(*Config)


// Stat: 返回当前对列的状态
q.Stat()

// Push: 入队
err := q.Push(ctx)
if err != nil {
	// ... 对列满,被排除/被裁决
}


// Pop: 出队
q.Pop()

```

## downgrade

熔断降级

## egroup

组件生命周期管理
```go
// errorGroup 
// 级联控制,如果有组件发生错误,会通知group所有组件退出
// 声明声明周期管理
var admin = egroup.NewLifeAdmin()

srv := &http.Server{
		Addr: ":8080",
}
// 增加任务
admin.Add(egroup.Member{
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
// admin.Start() 启动
fmt.Println("error", admin.Start())
defer admin.shutdown()
```


## errors

封装一些error处理

## goroutine

池化,控制野生goroutine

```go
g := goroutine.NewGoroutine(context.Background())
// 改变 pool 上限
g.ChangeMax(n)

// 添加异步任务,内部会调用协程
// 如果 返回 false 可能代表任务已经满了 直接丢弃
// 这部分逻辑需要参考 
g.AddTast(func ()) bool

// 关闭池,回收资源
g.Shutdown() 
```

## log

日志相关

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
// 建立Group 中间件
middle := bbr.NewLimiter()

// 在middleware中 
// ctx中携带这两个可配置的有效数据
// 可以通过 ctx.Set

// 配置获取限制器类型,可以根据不同api获取不同的限制器
ctx := context.WithValue(ctx,bbr.LimitKey,"key")

// 可配置成功是否上报
// 必须是 overload.Op 类型
ctx := context.WithValue(ctx,bbr.LimitOp,overload.Success)
```

## restrictor

限流器

### rate

漏桶

```go
// 第一个参数是 r Limit。代表每秒可以向 Token 桶中产生多少 token。Limit 实际上是 float64 的别名
// 第二个参数是 b int。b 代表 Token 桶的容量大小。
// limit := Every(100 * time.Millisecond);
// limiter := rate.NewLimiter(limit, 4)
// 以上就表示每 100ms 往桶中放一个 Token。本质上也就是一秒钟产生 10 个。

// rate: golang.org/x/time/rate
limiter := rate.NewLimiter(2, 4)
// rate1: Gkit/rate
af, wf := rate1.NewRate(limiter)

// af.Allow()bool: 默认取1个token
// af.Allow() == af.AllowN(time.Now(), 1)
af.Allow()

// af.AllowN(ctx,n)bool: 可以取N个token
af.AllowN(time.Now(), 5)

// wf.Wait(ctx) err: 等待ctx超时,默认取1个token
// wf.Wait(ctx) == wf.WaitN(ctx, 1) 
wf.Wait(ctx)

// wf.WaitN(ctx, n) err: 等待ctx超时,可以取N个token
wf.WaitN(ctx, N)

```
### ratelimite

令牌桶

```go
// ratelimit:github.com/juju/ratelimit
bucket := ratelimit.NewBucket(time.Second/2, 4)

// ratelimite2: Gkit.ratelimite
af, wf := ratelimite2.NewRateLimit(bucket)

//... 其他与漏桶使用一致
```
## timeout

各个服务间的超时控制

```go
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
```

## window

提供指标窗口
```go
// 初始化窗口
w := window.InitWindow()

// 增加指标
// key:权重
w.AddIndex(key, Score)

// Show: 返回当前指标
slice := w.Show()
```