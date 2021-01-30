package errors

// Cancelled: 请求被客户端取消。
// HTTPCode: 499
func Cancelled(format string, a ...interface{}) error {
	return Errorf(1, format, a...)
}

// IsCancelled:
func IsCancelled(err error) bool {
	return IsError(1, err)
}

// Unknown: 出现未知的服务器错误。通常是服务器错误
// HTTPCode: 500
func Unknown(format string, a ...interface{}) error {
	return Errorf(2, format, a...)
}

// IsUnknown:
func IsUnknown(err error) bool {
	return IsError(2, err)
}

// InvalidArgument: 客户端指定了无效参数
// HTTPCode: 400
func InvalidArgument(format string, a ...interface{}) error {
	return Errorf(3, format, a...)
}

// IsInvalidArgument:
func IsInvalidArgument(err error) bool {
	return IsError(3, err)
}

// DeadlineExceeded: 超出请求时限
// HTTPCode: 504
func DeadlineExceeded(format string, a ...interface{}) error {
	return Errorf(4, format, a...)
}

func IsDeadlineExceeded(err error) bool {
	return IsError(4, err)
}

// NotFound: 找不到指定的资源，或者请求由于未公开的原因（例如白名单）而被拒绝
// HTTPCode: 404
func NotFound(format string, a ...interface{}) error {
	return Errorf(5, format, a...)
}

// IsNotFound
func IsNotFound(err error) bool {
	return IsError(5, err)
}

// AlreadyExists: 客户端尝试创建的资源已存在。
// HTTPCode: 409
func AlreadyExists(format string, a ...interface{}) error {
	return Errorf(6, format, a...)
}

// IsAlreadyExists
func IsAlreadyExists(err error) bool {
	return IsError(6, err)
}

// PermissionDenied: 客户端权限不足。可能的原因包括 OAuth 令牌的覆盖范围不正确、客户端没有权限或者尚未为客户端项目启用 API
// HTTPCode: 403
func PermissionDenied(format string, a ...interface{}) error {
	return Errorf(7, format, a...)
}

// IsPermissionDenied
func IsPermissionDenied(err error) bool {
	return IsError(7, err)
}

// ResourceExhausted: 资源配额不足或达到速率限制
// HTTPCode: 429
func ResourceExhausted(format string, a ...interface{}) error {
	return Errorf(8, format, a...)
}

// IsResourceExhausted
func IsResourceExhausted(err error) bool {
	return IsError(8, err)
}

// FailedPrecondition: 请求无法在当前系统状态下执行，例如删除非空目录
// HTTPCode: 400
func FailedPrecondition(format string, a ...interface{}) error {
	return Errorf(9, format, a...)
}

// IsFailedPrecondition
func IsFailedPrecondition(err error) bool {
	return IsError(9, err)
}

// Aborted: 并发冲突，例如读取/修改/写入冲突
// HTTPCode: 409
func Aborted(format string, a ...interface{}) error {
	return Errorf(10, format, a...)
}

// IsAborted
func IsAborted(err error) bool {
	return IsError(10, err)
}

// OutOfRange: 客户端指定了无效范围
// HTTPCode: 400
func OutOfRange(format string, a ...interface{}) error {
	return Errorf(11, format, a...)
}

// IsOutOfRange
func IsOutOfRange(err error) bool {
	return IsError(11, err)
}

// Unimplemented: 此服务未实现或未支持/启用该操作
// HTTPCode: 501
func Unimplemented(format string, a ...interface{}) error {
	return Errorf(12, format, a...)
}

// IsUnimplemented
func IsUnimplemented(err error) bool {
	return IsError(12, err)
}

// Internal: 出现内部服务器错误。通常是服务器错误
// HTTPCode: 500
func Internal(format string, a ...interface{}) error {
	return Errorf(13, format, a...)
}

// IsInternal
func IsInternal(err error) bool {
	return IsError(13, err)
}

// Unavailable: 服务不可用。通常是服务器已关闭
// HTTPCode: 503
func Unavailable(format string, a ...interface{}) error {
	return Errorf(14, format, a...)
}

// IsUnavailable
func IsUnavailable(err error) bool {
	return IsError(14, err)
}

// DataLoss: 出现不可恢复的数据丢失或数据损坏。客户端应该向用户报告错误
// HTTPCode: 500
func DataLoss(format string, a ...interface{}) error {
	return Errorf(15, format, a...)
}

// IsDataLoss
func IsDataLoss(err error) bool {
	return IsError(15, err)
}

// Unauthenticated: 由于 OAuth 令牌丢失、无效或过期，请求未通过身份验证
// HTTPCode: 401
func Unauthenticated(format string, a ...interface{}) error {
	return Errorf(16, format, a...)
}

// IsUnauthenticated:
func IsUnauthenticated(err error) bool {
	return IsError(16, err)
}
