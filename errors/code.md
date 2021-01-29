## google.rpc.Code
| HTTP | RPC                   | 说明                                                         |
| :--- | :-------------------- | :----------------------------------------------------------- |
| 200  | `OK`                  | 无错误。                                                     |
| 400  | `INVALID_ARGUMENT`    | 客户端指定了无效参数。如需了解详情，请查看错误消息和错误详细信息。 |
| 400  | `FAILED_PRECONDITION` | 请求无法在当前系统状态下执行，例如删除非空目录。             |
| 400  | `OUT_OF_RANGE`        | 客户端指定了无效范围。                                       |
| 401  | `UNAUTHENTICATED`     | 由于 OAuth 令牌丢失、无效或过期，请求未通过身份验证。        |
| 403  | `PERMISSION_DENIED`   | 客户端权限不足。可能的原因包括 OAuth 令牌的覆盖范围不正确、客户端没有权限或者尚未为客户端项目启用 API。 |
| 404  | `NOT_FOUND`           | 找不到指定的资源，或者请求由于未公开的原因（例如白名单）而被拒绝。 |
| 409  | `ABORTED`             | 并发冲突，例如读取/修改/写入冲突。                           |
| 409  | `ALREADY_EXISTS`      | 客户端尝试创建的资源已存在。                                 |
| 429  | `RESOURCE_EXHAUSTED`  | 资源配额不足或达到速率限制。如需了解详情，客户端应该查找 google.rpc.QuotaFailure 错误详细信息。 |
| 499  | `CANCELLED`           | 请求被客户端取消。                                           |
| 500  | `DATA_LOSS`           | 出现不可恢复的数据丢失或数据损坏。客户端应该向用户报告错误。 |
| 500  | `UNKNOWN`             | 出现未知的服务器错误。通常是服务器错误。                     |
| 500  | `INTERNAL`            | 出现内部服务器错误。通常是服务器错误。                       |
| 501  | `NOT_IMPLEMENTED`     | API 方法未通过服务器实现。                                   |
| 503  | `UNAVAILABLE`         | 服务不可用。通常是服务器已关闭。                             |
| 504  | `DEADLINE_EXCEEDED`   | 超出请求时限。仅当调用者设置的时限比方法的默认时限短（即请求的时限不足以让服务器处理请求）并且请求未在时限范围内完成时，才会发生这种情况。 |


## 错误生成

| HTTP | RPC                   | 错误消息示例                                        |
| :--- | :-------------------- | :-------------------------------------------------- |
| 400  | `INVALID_ARGUMENT`    | 请求字段 x.y.z 是 xxx，预期为 [yyy, zzz] 内的一个。 |
| 400  | `FAILED_PRECONDITION` | 资源 xxx 是非空目录，因此无法删除。                 |
| 400  | `OUT_OF_RANGE`        | 参数“age”超出范围 [0,125]。                         |
| 401  | `UNAUTHENTICATED`     | 身份验证凭据无效。                                  |
| 403  | `PERMISSION_DENIED`   | 使用权限“xxx”处理资源“yyy”被拒绝。                  |
| 404  | `NOT_FOUND`           | 找不到资源“xxx”。                                   |
| 409  | `ABORTED`             | 无法锁定资源“xxx”。                                 |
| 409  | `ALREADY_EXISTS`      | 资源“xxx”已经存在。                                 |
| 429  | `RESOURCE_EXHAUSTED`  | 超出配额限制“xxx”。                                 |
| 499  | `CANCELLED`           | 请求被客户端取消。                                  |
| 500  | `DATA_LOSS`           | 请参阅注释。                                        |
| 500  | `UNKNOWN`             | 请参阅注释。                                        |
| 500  | `INTERNAL`            | 请参阅注释。                                        |
| 501  | `NOT_IMPLEMENTED`     | 方法“xxx”未实现。                                   |
| 503  | `UNAVAILABLE`         | 请参阅注释。                                        |
| 504  | `DEADLINE_EXCEEDED`   | 请参阅备注。                                        |




| HTTP | RPC                   | 建议的错误详细信息               |
| :--- | :-------------------- | :------------------------------- |
| 400  | `INVALID_ARGUMENT`    | `google.rpc.BadRequest`          |
| 400  | `FAILED_PRECONDITION` | `google.rpc.PreconditionFailure` |
| 400  | `OUT_OF_RANGE`        | `google.rpc.BadRequest`          |
| 401  | `UNAUTHENTICATED`     |                                  |
| 403  | `PERMISSION_DENIED`   |                                  |
| 404  | `NOT_FOUND`           | `google.rpc.ResourceInfo`        |
| 409  | `ABORTED`             |                                  |
| 409  | `ALREADY_EXISTS`      | `google.rpc.ResourceInfo`        |
| 429  | `RESOURCE_EXHAUSTED`  | `google.rpc.QuotaFailure`        |
| 499  | `CANCELLED`           |                                  |
| 500  | `DATA_LOSS`           |                                  |
| 500  | `UNKNOWN`             |                                  |
| 500  | `INTERNAL`            |                                  |
| 501  | `NOT_IMPLEMENTED`     |                                  |
| 503  | `UNAVAILABLE`         |                                  |
| 504  | `DEADLINE_EXCEEDED`   |                                  |

