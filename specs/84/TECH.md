# Issue 84：技术方案

## Context

行为来源为 [PRODUCT.md](./PRODUCT.md)。修改限定在 `timeout/`、`tools/deepcopy`、`tools/vto`、`tools/bind`、`tools/match`、`tools/stm`、`tools/reflect2value`、`coding/{json,xml,yaml}` 与 `ternary`。

## Proposed changes

### A. Timeout（Behavior 1–3、12）

- 将 `Stamp.Value` 改为标准 `(driver.Value, error)`，增加编译期接口断言；`Scan` 增加 `int64` 和 default error。
- Date/DateTime/DTime 的文本解析统一用 `time.ParseInLocation(..., time.Local)`；序列化格式不变。
- DateStruct/DateTimeStruct 的 `MarshalJSON` 改为值接收者，使值与指针都覆盖内嵌 `time.Time` 的 RFC3339 marshal。

### B. Deepcopy 与转换（Behavior 4–5、9、11）

- struct copy 将不可寻址 source 转为临时可寻址值，并通过最小受控的 `reflect.NewAt` + `unsafe` 仅访问同类型的未导出字段，使普通私有值与私有引用也进入 visited graph 深拷贝；`time.Time` 作为稳定值整体保留，`sync`/`sync/atomic` 原语保持零值，`gkit:"-"` 仍保留 destination 原值。
- vto 在指针解引用后先检查 `IsValid`。
- stm 在 typed-nil 指针 `Elem` 前返回 nil。
- reflect2value 的所有 slice 分支在 `.Len()` 前验证 Slice/Array kind，并走现有 type-conversion error。

### C. Binding 与 codecs（Behavior 6–7、10）

- protobuf binder 使用 checked type assertion，并在 unmarshal 前对所有 nilable kind 统一拒绝 typed-nil message。
- form mapping 沿用“字段是否实际写入”的返回信息：nil pointer 先在临时候选值递归，只有子树命中输入时才写回；递归路径同时跟踪 nil pointer 类型与既有 pointer 的类型/地址，分别截断类型环和对象环。
- JSON/XML/YAML 在顶层 value 不可设置且为 nil pointer 时直接返回目标错误；仅对可设置的嵌套 pointer 执行分配循环。

### D. Match 与 ternary（Behavior 8、13）

- rune 路径比较 decoded rune `sr` 与 `utf8.RuneError`，不再把字节宽度与 rune 常量比较。
- ternary generator 在 package 声明后输出 `import "time"`，再执行模板与 `format.Source`。

## Behavior-to-test mapping

1. Stamp Valuer 接口断言与对应 `time.Time` Value。
2. Stamp 各 Scan 输入及 unsupported error。
3. 临时设置 `time.Local` 为固定 UTC+8，分别验证三类 `Value → Scan`。
4. 同时覆盖 `Clone(valueStruct)`、`time.Time`、`math/big.Int` 内部 slice、带锁 struct 的私有标量/引用 alias independence 与锁状态重置。
5. VoToDo 两条入口的 nil pointer field。
6. protobuf 非 message、typed-nil message error 与合法 control。
7. 在子进程/受控测试中绑定 nil 自引用类型及预先存在的对象环并及时返回，同时覆盖实际键能分配一层和既有非环链正常遍历。
8. ASCII/中文源的相同尾随 `?` 矩阵。
9. StructToMap typed-nil 与 non-nil control。
10. 三种 codec 的 typed-nil error 和 `**T` allocation control。
11. 每个支持的 slice 类型至少用一个 scalar 错误输入，并覆盖合法 slice。
12. DateStruct/DateTimeStruct 值和指针 JSON 相等且可反序列化。
13. 在临时 module 中运行生成器输出并 `go test`/`go build`。

## Verification

```bash
gofmt -w <changed-go-files>
GOTOOLCHAIN=go1.20.14 go test -race -count=1 ./timeout ./tools/deepcopy ./tools/vto ./tools/bind ./tools/match ./tools/stm ./tools/reflect2value ./coding/json ./coding/xml ./coding/yaml ./ternary
GOTOOLCHAIN=go1.20.14 go vet ./timeout ./tools/deepcopy ./tools/vto ./tools/bind ./tools/match ./tools/stm ./tools/reflect2value ./coding/json ./coding/xml ./coding/yaml ./ternary
git diff --check
```

每项先记录旧实现 red，再做局部修复；对 Stamp、timezone round-trip、deepcopy、self-reference bind、typed-nil codec、slice kind 与 generator import 做恢复式 mutation。自引用旧实现可能触发不可恢复 stack overflow，red 证据必须在隔离子进程完成。
