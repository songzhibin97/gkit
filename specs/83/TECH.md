# Issue 83：技术方案

## Proposed changes

### TCP（Behavior 1–3）

- Send 维护 offset，按 retry 策略重试 `data[offset:]`，零进展/最终错误带上下文返回。
- 固定长度读使用 exact-read 语义；流式读在收到首段后用临时 idle deadline 继续 drain，defer 恢复原 deadline。RecvLine 对空返回先判断 error/length。
- 用 net.Pipe/fake net.Conn 确定性测试 partial write、segmented read、early EOF 与 stale deadline。

### Registry（Behavior 4–6）

- GetServices 用显式 scanned-row 计数实现 do-while 行覆盖。
- Subset 入口处理非正 size。
- AddService 先查重；每行维护/扫描 nil 槽优先复用，remove 清除该服务的唯一位置并更新索引。

### IP/ARP（Behavior 7–8）

- IPv6 使用标准库 `IsPrivate`、`IsLinkLocalUnicast`、`IsLinkLocalMulticast` 与 loopback 判定。
- pcap OpenLive 使用有限 read timeout；NextErrorTimeoutExpired 回到总 deadline 循环。

### BBR（Behavior 9–10）

- `winBucketPerSec` 下限钳为 1。
- middleware 在 handler 正常返回后统一调用 Success 完成函数，业务 error 原样返回；只有 limiter 拒绝和非正常路径维持 Drop 语义，核心 Op gate 不改。

## Tests

1–3. fake conn/net.Pipe 覆盖 RecvLine EOF、partial Send、exact early EOF、two-segment stream 与 internal deadline restoration；逐项 mutation 必须失败。
4. 1-client/3-service 与 3-client/3-service 均覆盖所有行。
5. size 0/negative 空结果和正数 control。
6. 重复 add/remove churn 后矩阵长度有界、无幽灵、hole 被复用。
7. `fc00::1`、`fd*`、`fe80::1`、link-local multicast 与 global IPv6 matrix。
8. ARP read timeout 逻辑抽成可注入/纯循环测试；真实 pcap 只作环境允许时的集成证据。
9. 5m window/100 buckets 的 maxFlight 正数。
10. middleware 业务 error 更新 pass/rt；显式 Drop control 仍不更新。

## Verification

```bash
GOTOOLCHAIN=go1.20.14 go test -race -count=1 ./net/tcp ./registry ./net/ip ./net/arp ./overload/bbr
GOTOOLCHAIN=go1.20.14 go vet ./net/tcp ./registry ./net/ip ./net/arp ./overload/bbr
git diff --check
```
