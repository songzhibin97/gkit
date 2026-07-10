# Issue 85：运行时诊断与数据完整性修复

## Summary

修复监控 dump、跨协议错误转换、trace peer、protobuf 嵌套命名、Snowflake 节点提取与 page token 防篡改中的确定性错误，使诊断输出保持原文、并发配置无竞态、协议元数据不丢失，且安全令牌在解密前经过完整性认证。

## Behavior

1. watching 的 goroutine、memory、thread、CPU 与 GC heap 自动 dump 触发日志都按 `UniformLogFormat` 渲染，不产生 `%!(EXTRA...)`，并保留 dump 动作、类型、配置阈值以及 previous/current 观测值。
2. watching 将文本 pprof 内容作为普通文本返回；内容中的 `%` 不会被当作格式动词改写。
3. `trimResult` 在输入不超过上限时保留全部段落；超过上限时只保留前 `TrimResultTopN` 段。
4. 运行中的 watching 可并发启停 thread、goroutine、CPU、memory 与 GC heap dump；配置读写在 `-race` 下无数据竞争。
5. `errors.FromError` 转换不带 `ErrorInfo` detail 的标准 gRPC status 时保留映射后的原始 gRPC code 与 message；本库带 detail 的 reason/metadata 行为保持不变。
6. trace 的 `parseTarget` 对带 scheme 的网络 endpoint 返回 `host:port`，对裸 `host:port` 保持同样结果，使 `peerAttr` 可生成 peer IP/port；既有 unix path 处理保持兼容。
7. protobuf parser 对任意深度的嵌套 message/enum 都按从外到内的父级前缀命名，例如 `A.B.C` 生成 `ABC`，不会生成反序前缀。
8. `IpToUint16` 对 `net.ParseIP` 返回的 16 字节 IPv4 表示先归一化为 IPv4，再从后两段生成节点值；nil、短 IP 与纯 IPv6 返回明确错误而不是 panic 或静默返回 0。
9. 新生成的 page token 使用带认证的加密；任何 nonce、密文或认证标签位翻转都返回 `ErrInvalidToken`。旧 CBC page token 不再被接受。`encrypt/aes` 保留既有 CBC API 以兼容普通调用方，并新增供安全边界使用的 AEAD API。

## Non-goals

- 不改变 watching 的阈值算法、采样周期、dump 文件格式或 reporter 协议。
- 不重命名 protobuf 顶层类型，也不改变字段类型映射。
- 不改变 Snowflake ID 位布局。
- 不把既有 `aes.Encrypt` / `aes.Decrypt` 的密文格式原地迁移；它们作为 legacy CBC API 保留，但不得继续承载 page token 完整性边界。
