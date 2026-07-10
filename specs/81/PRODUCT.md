# Issue 81：系统原语与 cgroup 跨平台正确性

## Summary

修复 mutex/token 互斥、Unicode rotation、32 位哈希编译与 cgroup v2 CPU 采样，使相同 API 在受支持架构和现代容器层级中给出一致、有限且可验证的结果。

## Behavior

1. 公共和 internal `Mutex.Count` 返回等待者数加当前持有者（0 或 1）；无等待者的已锁 mutex 返回 1。
2. `TokenRecursiveMutex` 接受包括 0 在内的任意 int64 token；空闲状态与 token 值独立，token 0 首次获取底层锁、同 token 递归计数、完全释放后互斥仍正确。公共/internal 副本一致且无 race。
3. `stringx.Rotate` 以 rune 数规范化 shift，多字节字符串与 ASCII 使用相同字符语义。
4. `sys/stringx` 在 linux/386 可编译，并保持现有 substring 行为。
5. `sys/xxhash3` 在 linux/386 可编译；同一输入的 64/128 位 hash 与 64 位平台结果一致，不发生 uintptr 截断。
6. 公共和 internal CPU reader 支持 cgroup v1 与 unified cgroup v2：v2 从 `cpu.max`、`cpu.stat` 和 `cpuset.cpus.effective`（必要时回退 `cpuset.cpus`）读取 quota、usage 与 CPU set，而不是静默回退宿主机指标。
7. cgroup CPU usage 在 quota 为零、system/usage counter 未前进或回退时返回 0，不产生 Inf 转换或 uint64 下溢；正常单调样本保持原公式。

## Non-goals

- 不在本次替换 `sys/syncx.Pool` 的 runtime cleanup 机制。runtime 只提供进程级单槽私有 hook，多套 linkname 实现无法安全组合；该限制作为不受支持的共存契约记录，不能用不可靠 chaining 假装修复。
- 不承诺 `Mutex.Count` / `IsLocked` 等 unsafe 状态内省跨任意未来 Go runtime ABI。`TryLock` 可委托标准库，但状态读取仍只在项目声明的 Go 1.20 支持线与当前验证版本使用；未来移除内省需要单独 API 决策。
- 不改变 xxhash3 算法输出或 cgroup v1 文件语义。
