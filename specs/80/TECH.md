# Issue 80：技术方案

## Context

行为来源为 [PRODUCT.md](./PRODUCT.md)。当前实现的关键问题分布如下：

- `container/pool/list.go` 混用原子与普通方式访问活跃计数，以无缓冲通知 channel 做非阻塞发送，并让空闲 cleaner 的启动和周期固定在构造时。
- `container/queue/codel/queue.go` 为每个 packet 从池中取得无缓冲裁决 channel；`Pop` 在发送方尚未等待时会丢弃裁决并提前复用 channel。
- `egroup/liftcycle.go` 用已经取消的 group context 调成员 `Shutdown`；`egroup/errgroup.go` 在取消前等待，并未串行化关闭判定与 WaitGroup 登记。
- `window/leap_array.go` 让外部 builder 原地修改已发布 bucket；`window/array.go` 以固定 8 字节计算指针槽位。
- `container/group/group.go` 将 factory 替换和缓存清空拆成两个临界区。
- `concurrent/or_done.go` 对单 channel 直接返回源 channel，而多 channel 只产生完成信号。

本 issue 的范围仅为 PRODUCT Behavior §1–§17。`FanInRec`、`MergeChannel`、`Pipeline` 与 `FanOut` 的新取消、顺序和背压设计是 Non-goals，不修改其代码或签名。

## Proposed changes

### A. Pool（Behavior §1–§5）

目标文件：`container/pool/list.go`、`container/pool/pool.go`，回归测试放在 `container/pool/`。

- 活跃计数的所有并发读取统一使用原子 load；容量 check 与 increment 继续在 `mu` 下形成一个决策，释放路径使用与调用点锁状态明确区分的 helper。
- 用 `mu` 保护的 generation channel 取代非阻塞发送：等待者在持锁时捕获当前 channel，状态变化在持锁时关闭并替换该 channel，等待者醒来后循环重新检查关闭、idle 和容量状态。这样通知发生在解锁后、真正 select 前也不会丢失，并可同时唤醒多个等待者。
- 保留现有 `SetWait(false, 0)` 立即返回耗尽、正 `waitTimeout` 受该超时限制的行为；仅为 `SetWait(true, 0)` 直接等待 generation channel 或调用方 context，不创建零时长 timeout。
- cleaner 使用可重设且总会 `Stop` 的 timer，并有独立 wake、stop、done 生命周期。`Reload` 在 idle timeout 的 0↔正数或正数变化时唤醒/启动 cleaner；禁用时停止 timer但保留重载能力。`Shutdown` 发 stop、广播池关闭并等待 cleaner done 后返回。
- 不改变 `Pool` 接口、资源 factory/Put 签名或重复关闭的现有错误值。

### B. CoDel（Behavior §6–§7）

目标文件：`container/queue/codel/queue.go`，回归测试放在 `container/queue/codel/`。

- 每次 `Push` 分配一个容量为 1 的 one-shot decision channel，并随 packet 传递；移除 decision channel 的 `sync.Pool` 复用。
- `Pop` 将一次裁决写入该 buffered channel。发送不依赖 `Push` 是否已 park；若 `Push` 已因 context 返回，buffer 仍吸收该裁决并随 packet 生命周期回收，不会阻塞或流入后续请求。
- 保持队列容量、CoDel 算法、`Push`/`Pop` 导出签名和错误类型不变。

### C. Egroup（Behavior §8–§11）

目标文件：`egroup/liftcycle.go`、`egroup/errgroup.go`，回归测试放在 `egroup/`。

- 成员关闭使用新的 `context.Background()` 作为 `Delegate` 父 context，再由现有 `stopTimeout` 施加 deadline；不复用已取消的 group context。`Delegate` wrapper 仍被 group 任务计数，因此 `Start` 等待关闭完成或超时。
- `Group` 增加内部状态锁，同一临界区完成“是否已关闭”判断与 `wg.Add(1)`；任务通过底层已有的 `AddTaskN` 与 group context 登记，使关闭 cancel 能解除尚未完成的阻塞登记。`Shutdown` 在同一锁下切换关闭状态并调用 cancel，释放锁后再 `wg.Wait()`，最后关闭底层 goroutine group。
- `AddTask` 拒绝时平衡 WaitGroup 计数，并通过现有 first-error 路径记录 `ErrGroupClosed` 与触发取消，使 `Wait` 返回可观察错误；不新增导出方法或错误类型。
- 保持 first-error、重复 `Shutdown` 和关闭后 `Go` 静默不接收的既有外部形态。

### D. Window（Behavior §12–§13）

目标文件：`window/array.go`、`window/bucket.go`、`window/leap_array.go`，回归测试放在 `window/`。

- AtomicArray 不再保存/计算 slice 基址与固定 stride；对规范化后的 index 直接取得 `&data[index]`，在该槽位执行 atomic load/CAS。
- LeapArray 跨周期时创建未发布 candidate bucket：用 `builder.NewEmptyBucket()` 初始化全新的 Value，在 candidate 上调用外部 `Reset`，完成后以 CAS 原子替换旧指针。candidate 不共享旧 bucket 的 Value 引用，因此即使 builder 原地清理 value，也不会写到读者仍在使用的旧状态。
- 不复制已使用的 `atomic.Value` 或复用其底层可变对象，不要求第三方 builder 改用原子写，也不改变 `BucketBuilder` 导出接口。
- 将现有指针大小断言改为基于本机 `unsafe.Sizeof(uintptr(0))`，使同一测试可在 32 位执行。

### E. Group（Behavior §14）

目标文件：`container/group/group.go`，回归测试放在 `container/group/`。

- `ReSet` 在一个写锁临界区内同时替换 factory 和替换缓存 map；保留 nil factory panic、`Clear` 与所有导出签名。

### F. OrDone（Behavior §15–§17）

目标文件：`concurrent/or_done.go`，回归测试放在 `concurrent/`。

- 0 个输入直接返回已关闭 channel。
- 1 个及以上输入都由统一 waiter 等待第一次 receive-ready 事件，然后只关闭结果 channel，不发送选中的 payload。
- 保留 reflect select 对 nil channel 的标准语义：nil case 永不就绪；全部 nil 时 waiter 持续等待。
- 不修改其他 concurrent combinator。

## Testing and validation

### Behavior-to-test mapping

1. Behavior §1 → `TestListActiveCountConcurrentRelease`：并发 factory 失败与容量检查，在 `-race` 下无竞态，成功创建数不超过 active 上限。
2. Behavior §2 → `TestListWaitBroadcastCannotBeLost`：两个等待者在捕获等待 generation 后、进入 select 前归还两个资源，二者都及时成功而非等到 timeout。
3. Behavior §3 → `TestListWaitWithoutPoolTimeoutUsesCallerContext`：`SetWait(true, 0)` 不提前返回；归还资源时成功，只有 caller cancel 时返回其 context 错误。
4. Behavior §4 → `TestListCleanerFollowsReloadedIdleTimeout`：覆盖 0→正数、正数→0、禁用期间不清理及重新启用/缩短周期后清理。
5. Behavior §5 → `TestListShutdownStopsCleanerAndWakesWaiters`：`Shutdown` 等待 cleaner done、等待者返回 `ErrPoolClosed`，并复用现有 idle exactly-once 测试验证资源计数。
6. Behavior §6 → `TestQueueDecisionBufferedBeforePushWaits`：从内部 packets 取得已入队 packet，验证 decision channel 为 one-shot buffered，并在接收方可能 park 前写入裁决，原 `Push` 得到该裁决。
7. Behavior §7 → `TestQueueCanceledPushDoesNotBlockOrCrossWire`：取消第一个 `Push` 后再投递其裁决不会阻塞；第二个 packet 使用独立 channel并只收到自己的裁决。
8. Behavior §8 → `TestLifeAdminShutdownUsesFreshBoundedContext`：成员断言 context 初始未取消且有 stopTimeout deadline；`Start` 在 callback 完成前不返回，并另测超时边界。
9. Behavior §9 → `TestGroupShutdownCancelsBeforeWaiting`：任务等待 group cancellation，`Shutdown` 在测试 deadline 内返回；mutation 为恢复 wait-before-cancel 时确定超时。
10. Behavior §10 → `TestGroupGoShutdownRegistrationIsSerialized`：用可控底层 group 扩大交错，证明 shutdown cancel 能解除阻塞的 context-aware 登记，并重复并发 `Go`/`Shutdown`；在 `-race` 下无 panic/WaitGroup misuse，关闭后无新任务被接收。
11. Behavior §11 → `TestGroupRejectedTaskReturnsErrorFromWait`：拒绝型底层 group 不执行任务，`Wait` 返回 `ErrGroupClosed`。
12. Behavior §12 → `TestLeapArrayPublishesResetBucketAtomically`：builder 在 candidate 上分阶段写 Start/Value，同时读旧槽位；发布前只见完整旧值，发布后只见完整新值，`-race` 无报告。
13. Behavior §13 → `TestAtomicArrayUsesNativePointerStride`：每个 index 返回 `data[index]`；同一测试交叉编译并在 386 emulator/binfmt 可用时执行。
14. Behavior §14 → `TestGroupResetIsAtomicWithConcurrentGet`：受控并发 reset/get 下，每轮新 factory 对同 key 最多调用一次，且 reset 返回后缓存来自新 factory。
15. Behavior §15 → `TestOrDoneNoInputsIsClosedSignal`：立即 receive 到 closed 状态且无 payload。
16. Behavior §16 → `TestOrDoneSingleInputIsSignalOnly`：value/close 都只关闭结果，不转发值；nil 输入在测试观察窗内不触发。
17. Behavior §17 → `TestOrDoneMultipleInputsAndNilSemantics`：最先 value/close 的非 nil 输入关闭结果、无 payload；nil 不抢占，全部 nil 不触发。

### Red → green commands

每组先只加入对应测试并在原实现上记录失败，再实施该组最小修复并运行同一命令转绿：

```bash
go test -race -count=1 ./container/pool -run 'TestList(Active|Wait|Cleaner|Shutdown)'
go test -race -count=1 ./container/queue/codel -run 'TestQueue(Decision|Canceled)'
go test -race -count=1 ./egroup -run 'Test(LifeAdmin|Group)'
go test -race -count=1 ./window -run 'Test(LeapArrayPublishes|AtomicArrayUses)'
go test -race -count=1 ./container/group -run 'TestGroupResetIsAtomic'
go test -race -count=1 ./concurrent -run 'TestOrDone'
```

### Mutation / revert checks

- A：把 active load 恢复为普通读，`TestListActiveCountConcurrentRelease` 必须被 `-race` 杀死；把 generation capture 移到解锁后、移除 `wait=true && timeout=0` 分支、忽略 Reload wake 或 Shutdown stop，各自对应 §2–§5 测试必须失败。
- B：把 decision channel 改回无缓冲或复用同一个 channel，§6/§7 测试必须阻塞、超时或观察到 channel identity/裁决错误。
- C：把 shutdown parent 改回 group ctx、恢复 wait-before-cancel、移除状态锁或拒绝错误记录，§8–§11 对应测试必须失败；并发登记 mutation 同时跑 `-race`。
- D：恢复已发布 bucket 原地 Reset 时 §12 的 `-race` 测试必须失败；恢复 8 字节 stride 时 386 的 §13 测试必须返回错误槽位。
- E：恢复 `ReSet` 两段临界区时 §14 受控交错测试必须观察到同一 factory 重复创建。
- F：恢复单输入直接返回或向结果发送 selected payload 时 §16/§17 必须失败。

mutation 只在本地临时修改后运行，验证失败即恢复正确实现；不把 mutation 留在 diff 中。

### Final verification

```bash
git diff --name-only -- '*.go' | xargs gofmt -w
go test -race -count=1 ./concurrent ./container/group ./container/pool ./container/queue/codel ./egroup ./window
go vet ./concurrent ./container/group ./container/pool ./container/queue/codel ./egroup ./window
CGO_ENABLED=0 GOOS=linux GOARCH=386 go test -c ./window -o /tmp/gkit-window-386.test
```

若本机已有 386 emulator 或 Docker binfmt，再执行：

```bash
/tmp/gkit-window-386.test -test.run 'TestAtomicArrayUsesNativePointerStride' -test.v
```

无法执行交叉架构 binary 时，保留成功的 386 test-binary 构建记录并明确运行时证据缺口。最终还需确认 `git diff --check`、限定包全量 race 测试和 vet 均通过，且 diff 未触及 Non-goals 中的文件。
