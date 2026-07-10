# Issue 82：ZSet 层级与越界安全

## Summary

修复跳表删除后的层级维护和 rank 越界规范化，并同步 internal CPU 副本的跨平台防御，使 ZSet 不退化、不泄漏内部哨兵，平台能力缺失也不会造成 panic 或伪失败。

## Behavior

1. 删除节点后只收缩已经为空的跳表顶层；仍含节点的索引层保持可达，删除最后一个高层节点时空层会正确收缩。
2. `Range` / `RevRange` 的负索引按集合长度换算：过小起点限制为首元素，换算后仍为负的终点判空，超过集合尾部的起点返回空，过大终点限制为尾元素；任何输入都不会返回 `__HEADER`。内部 `GetNodeByRank` 对非正 rank 返回 nil。
3. `internal/sys/cpu` 在 gopsutil 返回 error 或空 slice 的平台不索引空结果；平台测试只断言可用指标，与公共 `sys/cpu` 副本保持一致。

## Non-goals

- 不改变 ZSet 的排序、同分值字典序或合法 rank 结果。
- 不清理 `internal/wyhash.byteToSlice`；逃逸分析已证明其不是悬垂指针 bug。
- 不把未被生产代码引用的 `internal/sys/cpu` 提升为新公共 API。
