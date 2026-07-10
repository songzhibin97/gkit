# Issue 80：并发与生命周期正确性

## Summary

修复 `concurrent`、`container`、`egroup` 与 `window` 中已确认的并发、等待、关闭和跨架构错误，使调用方在竞争、取消、重载与关闭条件下得到一致且可终止的结果。

## Behavior

1. 连接池在并发创建、创建失败、归还和强制关闭资源时始终遵守配置的活跃资源上限；失败或已关闭的资源不会永久占用容量，也不会导致并发访问崩溃或状态损坏。
2. 当连接池的资源或容量重新可用时，所有正在等待的调用都会重新检查池状态；通知即使发生在调用开始休眠之前也不会丢失，多个并发等待者不会因通知合并而在已有可用资源时虚假超时。
3. `SetWait(true, 0)` 表示不设置池内等待超时：`Get` 会等待资源可用或调用方 context 结束，并原样返回调用方 context 的错误。
4. 连接池的空闲清理配置可在运行时从禁用切换为启用、从启用切换为禁用或更改周期；每次重载后的行为以最新配置为准，禁用期间不清理，重新启用后过期资源会被清理。
5. 连接池 `Shutdown` 返回前停止内部空闲清理活动并唤醒等待者；等待者观察到池已关闭，空闲资源仍只关闭一次，重复 `Shutdown` 保持既有幂等错误语义。
6. CoDel 队列接受一个 `Push` 后，只要对应 packet 被 `Pop` 裁决，该 `Push` 就会收到自己的放行或拒绝结果；裁决不会因发送方尚未开始等待而丢失，也不会被其他请求接收。
7. CoDel `Push` 的 context 先结束时会返回该 context 的错误；稍后到达的裁决不会阻塞 `Pop`，也不会影响或解除后续 `Push`。
8. `LifeAdmin` 开始关闭成员时，成员收到的是尚未取消且受 `stopTimeout` 限制的 context；`Start` 会等待成员关闭完成或该超时到达，不会在关闭函数仍运行时提前返回。
9. `egroup.Group.Shutdown` 会先发出取消信号再等待已接收任务退出，因此等待 group context 的任务不会与 `Shutdown` 相互死锁。
10. `egroup.Group.Go` 与 `Shutdown` 并发时，关闭判定和任务登记具有单一顺序：关闭开始后不再接收新任务，已登记任务全部被计入等待，调用不会触发 WaitGroup 误用或 panic。
11. 底层 goroutine group 拒绝 `Go` 提交的任务时，任务不会执行，且调用方随后可从 `Wait` 观察到非 nil 错误，而不是得到成功结果。
12. LeapArray 跨周期重置时一次性发布完整的新 bucket；并发读取者只能看到完整的旧 bucket 或完整的新 bucket，不会看到新起始时间搭配旧值，也不会因调用方的 `BucketBuilder.Reset` 写入而发生并发状态损坏。
13. AtomicArray 在所有受支持的指针宽度上都按传入索引访问对应 bucket；32 位平台的非零索引不会读取相邻槽位或越界内存。
14. `Group.ReSet` 对调用方表现为一次原子操作：新 factory 生效时旧缓存已同时失效；同一轮 reset 的新 factory 不会因并发 `Get` 在清空前后为同一 key 创建两次。
15. `OrDone()` 不传 channel 时立即返回一个已关闭、且不产生 payload 的完成信号 channel。
16. `OrDone(ch)` 传一个 channel 时，在该 channel 第一次可读或关闭时关闭结果 channel；结果 channel 不转发源 payload，nil 源 channel 会像普通 nil channel 一样永不触发。
17. `OrDone(channels...)` 传多个 channel 时，在任一非 nil channel 第一次可读或关闭时关闭结果 channel且不转发 payload；nil channel 不会自行触发，全部为 nil 时结果保持未完成。

## Non-goals

- 不为 `FanInRec`、`MergeChannel`、`Pipeline` 或 `FanOut` 新增 context、取消或下游放弃协议。
- 不改变 `FanOut` async 模式的顺序、并发度或背压契约。
- 不扩大现有导出 API；本次只修复上述既有 API 的确定性错误。
