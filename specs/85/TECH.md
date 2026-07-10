# Issue 85：技术方案

## Context

行为来源为 [PRODUCT.md](./PRODUCT.md)。当前问题分布在 `watching/`、`errors/`、`trace/`、`parser/parse_pb/`、`generator/`、`page_token/` 与 `encrypt/aes/`，彼此没有共享状态，按行为做局部修复。

## Proposed changes

### A. Watching（Behavior 1–4）

- 五个自动 dump 调用统一把 `UniformLogFormat` 作为 `logf` 的 format 参数。
- `writeFile` 用普通 error 构造函数包装文本，不再把动态 pprof 文本交给格式化函数。
- `trimResult` 仅在段数超过上限时截断；不足时 join 全部段落。
- 所有 `Enable*Dump` / `Disable*Dump` setter 使用 `configs.L` 写锁，与 `Get*Configs` 读锁形成同一同步域；`GetGroupConfigs` 还需复制内嵌 `*typeConfig`，避免锁外读取仍指向共享配置。

### B. Errors 与 trace（Behavior 5–6）

- `FromError` 在 `status.FromError` 成功后，先保留现有 `ErrorInfo` detail 分支；没有该 detail 时以 `StatusFromGRPCCode(gs.Code())`、空 reason 与 `gs.Message()` 构造兼容 `Error`。
- `parseTarget` 在 URL 含 `Host` 时返回 `u.Host`；仅 unix/path 风格使用既有去前导斜线 fallback；裸 endpoint 继续通过补 scheme 解析。

### C. Parser 与 generator（Behavior 7–8）

- 嵌套 message 与 enum 的递归 prefix 都使用 `prefix + ms.Name`。
- `IpToUint16` 调 `To4()` 并校验结果；新增稳定 sentinel error，`LocalIpToUint16` 复用同一转换函数。

### D. Page token AEAD（Behavior 9）

- 在 `encrypt/aes` 新增 AES-GCM encrypt/decrypt API：随机 nonce 前置于 sealed ciphertext，base64 编码；所有 decode、key、长度与认证失败统一返回 `ErrDecryptionFailed`。
- `page_token.ForIndex` 与 `GetIndex` 只使用新 AEAD API。由于旧 CBC token 不带认证标签，迁移后会被拒绝；这是关闭完整性漏洞的有意兼容边界。
- 既有 CBC `Encrypt` / `Decrypt` 保持原实现和测试，避免无关调用方密文格式变化。

## Behavior-to-test mapping

1. Behavior 1 → watching 表驱动测试使用非零且互异的 min/diff/abs、previous/current（不支持 max 的类型保持约定值 0），逐一断言五个 dump call site 产生的第一条 trigger 日志完整等于 `UniformLogFormat`；后续重复日志不能掩盖任一 call site 或参数 mutation。
2. Behavior 2 → `writeFile` 文本包含 `50% of total`，断言错误文本逐字相同；恢复动态 format 时失败。
3. Behavior 3 → 分别输入 3 段、10 段和 11 段，断言全保留或准确截断；恢复 `len-1` 时失败。
4. Behavior 4 → 并发循环调用五组 setter 与 getter，在 `-race` 下运行；去掉任一同步域时 race detector 报告。
5. Behavior 5 → 用 `status.Error(codes.NotFound, "missing")` 断言 code 404/message；带 `ErrorInfo` control 继续保留 reason/metadata。
6. Behavior 6 → 覆盖 `grpc://127.0.0.1:9000`、`127.0.0.1:9000` 与 unix path，并让 scheme endpoint 成功生成 peer attributes。
7. Behavior 7 → 解析 `A { B { C {} enum D {} } }`，断言 keys 含 `A/AB/ABC/ABD` 且不含 `BAC/BAD`。
8. Behavior 8 → 覆盖 `net.ParseIP("192.168.2.3")` 得 515，以及 nil/短 slice/IPv6 error；恢复直接索引时值错或 panic。
9. Behavior 9 → AEAD round-trip、随机 nonce、wrong-key、合法 base64 的空/短 nonce/短 tag/截断 payload 与逐区域 bit-flip 均验证；所有 malformed payload 统一返回 `ErrDecryptionFailed` 且不 panic，page token bit-flip 必须 `ErrInvalidToken`，旧 CBC token control 必须被拒绝。

## Verification

```bash
gofmt -w <changed-go-files>
GOTOOLCHAIN=go1.20.14 go test -race -count=1 ./watching ./errors ./trace ./parser/parse_pb ./generator ./encrypt/aes ./page_token
GOTOOLCHAIN=go1.20.14 go vet ./watching ./errors ./trace ./parser/parse_pb ./generator ./encrypt/aes ./page_token
git diff --check
```

每组回归先在旧实现记录 red，再做最小修复；至少逐一 mutation 五个 dump call site 及其字段，并对 trim、setter lock、gRPC code、scheme target、nested prefix、IP normalization、AEAD tamper 与 GCM 长度 guard 做恢复式 mutation，确认失败后恢复最终实现。
