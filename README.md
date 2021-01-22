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

## errors

封装一些error处理

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

## log

日志输出

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
v, err, shade := g.Do(key, func)
if err != nil {
	// ...
}
// 判断数据有效后,将v 放到cache
cache(v)

// 异步调用
ch := g.DoChan(key, func)
v <- ch
// v.Val
// v.Err
// v.Shared


// 尽力取消
g.Forget(key)

```
## restrictor

限流器

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

## downgrade

熔断降级