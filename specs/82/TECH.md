# Issue 82：技术方案

## Proposed changes

- `deleteNode` 的层高循环改为仅在 `header.next(top) == nil` 时降层。
- `GetNodeByRank(rank <= 0)` 直接返回 nil；Range/RevRange 在取节点前完成负索引换算、起终点边界与空范围判断。
- 将公共 `sys/cpu` 已有的空 slice guard 与平台能力测试策略同步到 `internal/sys/cpu`。

## Tests

1. 手工构造确定性的两层跳表，删除一个两层节点后仍有高层节点，断言 `highestLevel` 和 top link 保留；再删除最后高层节点断言正确收缩。恢复反条件时第一步失败。
2. 三元素集合覆盖 `Range(-4,-1)`、`Range(0,-4)` 及其反向版本、超大 stop、空集合与 direct rank 0，断言无 header 且换算后仍为负的 stop 判空。
3. internal CPU 的 Usage/Info 空 slice 路径通过防御性 helper 覆盖，现有 `TestStat` 在指标不可用时跳过相应断言。

## Verification

```bash
GOTOOLCHAIN=go1.20.14 go test -race -count=1 ./structure/zset ./internal/sys/cpu
GOTOOLCHAIN=go1.20.14 go vet ./structure/zset ./internal/sys/cpu
git diff --check
```
