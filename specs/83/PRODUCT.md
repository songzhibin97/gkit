# Issue 83：网络、子集与 BBR 边界正确性

## Summary

修复 TCP 部分读写、registry 子集生命周期、IPv6/ARP 网络识别和 BBR 长窗口统计，使断连、分段、churn 与业务错误都不会造成 panic、重复字节、幽灵实例、永久阻塞或容量估计归零。

## Behavior

1. `RecvLine` 在对端于换行前关闭时返回已读部分和 EOF 类错误；空 EOF 不索引空 slice、不 panic。
2. `Send` 跟踪每次实际写入的字节，重试只发送剩余部分；任何最终写错误（包括 `io.EOF`）返回给调用方，不报告虚假成功。
3. `Recv(length>0)` 在提前 EOF 时返回部分数据与 `io.ErrUnexpectedEOF`/EOF；`Recv(-1)` 聚合跨多个 TCP segment 的数据直到 EOF 或短暂 idle timeout，并在返回前恢复调用前的 read deadline，后续读不被内部探测 deadline 污染。
4. RockSteadier `GetServices` 至少扫描每一行一次，单 client 和最后一行都不遗漏，在 `Lot` 内返回稳定且去重的服务。
5. `Subset` 对 `size<=0` 返回空结果，不 panic；正 size 行为保持不变。
6. RockSteadier 重复 `AddService` 幂等；删除会清除唯一位置并把槽位加入可复用空间，长期 add/remove 不无限增长，也不返回幽灵服务。
7. `HasLocalIPAddr` 将 IPv6 loopback、ULA/private 与 link-local 单播/组播视为本地地址，`ClientPublicIP` 不把它们当公网来源。
8. ARP router discovery 使用有限 capture read timeout；静默接口在总 deadline 内返回，而不是永久阻塞。
9. BBR 在 bucket duration 大于一秒时仍使用至少 1 的 bucket-per-second 因子，`maxFlight` 不因整数截断固定为 0。
10. BBR middleware 将已经执行完成的 handler（即使返回业务 error）计入 Success 容量/RT 统计；fail-fast、panic 或显式 `Drop` 路径仍不进入成功统计。

## Non-goals

- 不承诺当 `services > clients * Lot` 时所有服务同时获得连接槽；容量本身不足。保持稳定 subsetting/低 churn，不引入每次调用轮换。该上限必须在文档/规格中明确。
- 不删除 BBR 核心对显式 `Drop` 的 Success gate；只修 middleware 对“已完成业务错误”的错误分类。
- 不改变 TCP 导出方法签名或 retry option 结构。
