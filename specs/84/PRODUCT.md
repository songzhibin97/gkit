# Issue 84：时间、反射与生成工具正确性

## Summary

修复 DB 时间类型的驱动契约与往返、通用反射工具的 nil/类型/递归边界，以及 ternary 代码生成的稳定构建，使公开的 error 返回通道真正承载无效输入，而不是静默丢值、清零或 panic。

## Behavior

1. `timeout.Stamp` 的值和指针形态满足 `driver.Valuer`；`Value` 保持返回对应的 `time.Time`，并增加 nil error。
2. `Stamp.Scan` 接受驱动常见的 `int64`、`[]byte`、`string` 与 `time.Time`；不支持的类型返回 error，绝不静默保留旧值。
3. `Date`、`DateTime` 与 `DTime` 是无时区 wall-clock 类型：文本和 DB 值统一按 `time.Local` 解析，因此在非 UTC local zone 中 `Value → Scan` 保持相同 wall-clock 与瞬时时间。
4. `deepcopy.DeepCopy` 与 `Clone` 保留不可寻址源 struct 的导出及普通未导出状态，深拷贝包括 `math/big.Int` 内部存储在内的未导出引用；`time.Time` 保持完整值语义，`sync`/`sync/atomic` 同步原语重置而不复制使用中状态，且所有可深拷贝引用均不与源别名。
5. `VoToDo` 与带 `FieldBind` 的 `VoToDoPlus` 遇到 nil 源指针字段时跳过该字段，不 panic，也不破坏目标字段已有值。
6. protobuf body binder 对非 `proto.Message` 及 typed-nil message 目标返回明确错误；合法 message 仍正常反序列化。
7. form/query binder 处理自引用指针类型时只为实际出现输入的路径分配对象；空输入面对 nil 类型环或预先存在的对象指针环都不会无限递归或栈溢出，既有非环链仍正常遍历。
8. `match.Match` 对 ASCII 与多字节 rune 使用一致的 `?` 语义；源已耗尽时尾随 `?` 不会被错误接受。
9. `StructToMap` 接收 typed-nil 指针时返回 nil，不 panic；非 nil struct 行为保持不变。
10. JSON、XML 与 YAML codec 的顶层 typed-nil 解码目标返回 error，不 panic；可寻址的 `**T` 等嵌套指针仍可按既有行为分配并解码。
11. `ReflectValue` 的 slice 类型分支在输入不是 slice/array 时返回类型转换 error，不 panic；合法 slice/array 转换保持不变。
12. `DateStruct` 与 `DateTimeStruct` 的值、指针 JSON 表示一致，且两种形态都能按各自格式往返。
13. ternary 生成器的完整模板输出包含所需 `time` import，生成文件可直接格式化和编译，不依赖手工补 import。

## Non-goals

- 不为 DB wall-clock 类型增加 location 配置项；本次明确沿用进程的 `time.Local`。
- 不改变 `Stamp` 的秒单位或底层类型。
- 不扩大 bind 的标签语法或默认值规则。
- 不改变 deepcopy 对 `gkit:"-"` 字段的跳过契约。
- 不重写 ternary 模板内容或生成 API。
