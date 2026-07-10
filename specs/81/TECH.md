# Issue 81：技术方案

## Proposed changes

### Mutex（Behavior 1–2）

- 从同一个原始 mutex state 分别提取 waiter count 和 locked bit；公共/internal 同步修改。
- TokenRecursiveMutex 用独立状态锁保护 held、token 和 recursion，不再用 token 0 表示空闲；首次 owner 与递归路径不会绕过底层锁，公共/internal 副本保持一致且无数据竞争。

### String/hash 32 位（Behavior 3–5）

- Rotate 用 `utf8.RuneCountInString` 取模；open-ended substring 使用 `math.MaxInt`。
- xxhash3 所有 `length * prime64_*` 先把 length 转为 uint64。

### Cgroup（Behavior 6–7）

- 解析 `/proc/<pid>/cgroup` 时仅为空 controller 的 unified entry 构造 v2 路径；v1 保留既有的 `root/<controller>` 映射，避免从 controller 顺序或 membership 猜测 mountpoint。
- 把文件读取/解析拆为可用临时目录验证的纯路径逻辑：`cpu.max` 的 `max` 表示无 CFS quota；`cpu.stat` 读取 `usage_usec` 并转换为纳秒；cpuset 优先 effective。
- newCGroupCPU 根据层级计算 quota，Usage 除法前验证 quota 与两个 counter 单调性；公共/internal 保持字节级语义一致。

## Tests

1. 锁住 mutex 后 Count=1，并增加受控等待者验证 count；恢复移位后取 locked bit 时失败。
2. token 0/非零首次锁、递归、跨 token 阻塞和完全释放；恢复零 sentinel 时 TryLock/Unlock 回归失败。
3. Unicode Rotate shift 大于 rune count。
4. linux/386 stringx 编译。
5. linux/386 xxhash3 编译，并在可用 386 runtime/container 执行固定向量与 amd64/arm64 golden 对比。
6. 临时 cgroup v1/v2 fixture 覆盖 quota、usage、cpuset、`max` 与 fallback 文件。
7. 纯 usage 计算覆盖 quota 0、counter equal/reset 与正常样本。

## Verification

```bash
GOTOOLCHAIN=go1.20.14 go test -race -count=1 ./sys/mutex ./internal/sys/mutex ./sys/stringx ./sys/xxhash3 ./sys/cpu ./internal/sys/cpu
GOTOOLCHAIN=go1.20.14 go vet ./sys/mutex ./internal/sys/mutex ./sys/stringx ./sys/xxhash3 ./sys/cpu ./internal/sys/cpu
CGO_ENABLED=0 GOOS=linux GOARCH=386 GOTOOLCHAIN=go1.20.14 go test -c ./sys/stringx
CGO_ENABLED=0 GOOS=linux GOARCH=386 GOTOOLCHAIN=go1.20.14 go test -c ./sys/xxhash3
git diff --check
```
