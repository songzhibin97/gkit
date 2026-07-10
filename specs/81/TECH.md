# Issue 81：技术方案

## Proposed changes

### Mutex（Behavior 1–2）

- 从同一个原始 mutex state 分别提取 waiter count 和 locked bit；公共/internal 同步修改。
- TokenRecursiveMutex 用独立状态锁保护 held、token 和 recursion，不再用 token 0 表示空闲；首次 owner 与递归路径不会绕过底层锁，公共/internal 副本保持一致且无数据竞争。

### String/hash 32 位（Behavior 3–5）

- Rotate 用 `utf8.RuneCountInString` 取模；open-ended substring 使用 `math.MaxInt`。
- xxhash3 所有 `length * prime64_*` 先把 length 转为 uint64。

### Cgroup（Behavior 6–7）

- 同时解析 `/proc/<pid>/cgroup` 与 `/proc/<pid>/mountinfo` 的 root、mountpoint、filesystem type、mount/super options，并解码 mountinfo 的 `\\040`、`\\011`、`\\012`、`\\134` 路径转义。membership 必须位于 mount root 内，映射结果为 `mountpoint + relative(membership, root)`；多个可见 mount 选择 root 最具体者。
- cgroup v1 按 mount options 识别各 controller 并保留真实 membership 子路径；只要存在 v1 `cpu` membership 就使用 v1 CPU hierarchy，即使 hybrid 同时存在 unified entry。没有 v1 `cpu` 时才按 cgroup2 mount 映射 unified membership；已声明的 v1 CPU hierarchy 不可见时返回错误，不回退 v2。
- v2 `cpu.max` 的 `max` 表示该层无 CFS quota；从当前目录遍历到映射后的 cgroup2 mountpoint（含两端），用无溢出的交叉乘法比较 `quota/period` 并返回最严格的有限配对。任何层的文件读取、权限、缺失或格式错误都直接传播，遍历不得越过 mountpoint。
- `cpu.stat` 读取 `usage_usec` 并转换为纳秒；cpuset 优先 effective。
- newCGroupCPU 根据层级计算 quota，Usage 除法前验证 quota 与两个 counter 单调性；公共/internal 保持字节级语义一致。

## Tests

1. 锁住 mutex 后 Count=1，并增加受控等待者验证 count；恢复移位后取 locked bit 时失败。
2. token 0/非零首次锁、递归、跨 token 阻塞和完全释放；恢复零 sentinel 时 TryLock/Unlock 回归失败。
3. Unicode Rotate shift 大于 rune count。
4. linux/386 stringx 编译。
5. linux/386 xxhash3 编译，并在可用 386 runtime/container 执行固定向量与 amd64/arm64 golden 对比。
6. public/internal 对称临时 fixture 覆盖 v1 nested membership、任意 root/mountpoint 与 bind-root 映射、mountinfo 转义、hybrid 的 v1 CPU 选择、v2 child `max` + parent finite、跨 period 的最严格比值、mount 边界、祖先缺失错误，以及 quota/usage/cpuset fallback。
7. 纯 usage 计算覆盖 quota 0、counter equal/reset 与正常样本。

## Verification

PR head 重复运行 Issue 81 的确定性 CPU fixture；完整相关包 gate 同时在最新 master、PR #90 与本 PR 的 synthetic merge tree 上运行，覆盖 internal CPU 的相邻变更。

```bash
GOTOOLCHAIN=go1.20.14 go test -race -count=20 ./sys/cpu ./internal/sys/cpu -run TestIssue81
GOTOOLCHAIN=go1.20.14 go test -race -count=1 ./sys/mutex ./internal/sys/mutex ./sys/stringx ./sys/xxhash3 ./sys/cpu ./internal/sys/cpu
GOTOOLCHAIN=go1.20.14 go vet ./sys/mutex ./internal/sys/mutex ./sys/stringx ./sys/xxhash3 ./sys/cpu ./internal/sys/cpu
CGO_ENABLED=0 GOOS=linux GOARCH=386 GOTOOLCHAIN=go1.20.14 go test -c ./sys/stringx
CGO_ENABLED=0 GOOS=linux GOARCH=386 GOTOOLCHAIN=go1.20.14 go test -c ./sys/xxhash3
CGO_ENABLED=0 GOOS=linux GOARCH=386 GOTOOLCHAIN=go1.20.14 go test -c ./sys/cpu
CGO_ENABLED=0 GOOS=linux GOARCH=386 GOTOOLCHAIN=go1.20.14 go test -c ./internal/sys/cpu
git diff --check
```
