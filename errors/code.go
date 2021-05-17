package errors

// Cancelled 请求被客户端取消。
// HTTPCode 499
func Cancelled(reason, format string, a ...interface{}) error {
	return Errorf(1, reason, format, a...)
}

func IsCancelled(err error) bool {
	return IsError(1, err)
}

// Unknown 出现未知的服务器错误。通常是服务器错误
// HTTPCode 500
func Unknown(reason, format string, a ...interface{}) error {
	return Errorf(2, reason, format, a...)
}

func IsUnknown(err error) bool {
	return IsError(2, err)
}

// InvalidArgument 客户端指定了无效参数
// HTTPCode 400
func InvalidArgument(reason, format string, a ...interface{}) error {
	return Errorf(3, reason, format, a...)
}

func IsInvalidArgument(err error) bool {
	return IsError(3, err)
}

// DeadlineExceeded 超出请求时限
// HTTPCode 504
func DeadlineExceeded(reason, format string, a ...interface{}) error {
	return Errorf(4, reason, format, a...)
}

func IsDeadlineExceeded(err error) bool {
	return IsError(4, err)
}

// NotFound 找不到指定的资源，或者请求由于未公开的原因（例如白名单）而被拒绝
// HTTPCode 404
func NotFound(reason, format string, a ...interface{}) error {
	return Errorf(5, reason, format, a...)
}

func IsNotFound(err error) bool {
	return IsError(5, err)
}

// AlreadyExists 客户端尝试创建的资源已存在。
// HTTPCode 409
func AlreadyExists(reason, format string, a ...interface{}) error {
	return Errorf(6, reason, format, a...)
}

func IsAlreadyExists(err error) bool {
	return IsError(6, err)
}

// PermissionDenied 客户端权限不足。可能的原因包括 OAuth 令牌的覆盖范围不正确、客户端没有权限或者尚未为客户端项目启用 API
// HTTPCode 403
func PermissionDenied(reason, format string, a ...interface{}) error {
	return Errorf(7, reason, format, a...)
}

func IsPermissionDenied(err error) bool {
	return IsError(7, err)
}

// ResourceExhausted 资源配额不足或达到速率限制
// HTTPCode 429
func ResourceExhausted(reason, format string, a ...interface{}) error {
	return Errorf(8, reason, format, a...)
}

func IsResourceExhausted(err error) bool {
	return IsError(8, err)
}

// FailedPrecondition 请求无法在当前系统状态下执行，例如删除非空目录
// HTTPCode 400
func FailedPrecondition(reason, format string, a ...interface{}) error {
	return Errorf(9, reason, format, a...)
}

func IsFailedPrecondition(err error) bool {
	return IsError(9, err)
}

// Aborted 并发冲突，例如读取/修改/写入冲突
// HTTPCode 409
func Aborted(reason, format string, a ...interface{}) error {
	return Errorf(10, reason, format, a...)
}

func IsAborted(err error) bool {
	return IsError(10, err)
}

// OutOfRange 客户端指定了无效范围
// HTTPCode 400
func OutOfRange(reason, format string, a ...interface{}) error {
	return Errorf(11, reason, format, a...)
}

func IsOutOfRange(err error) bool {
	return IsError(11, err)
}

// Unimplemented 此服务未实现或未支持/启用该操作
// HTTPCode 501
func Unimplemented(reason, format string, a ...interface{}) error {
	return Errorf(12, reason, format, a...)
}

func IsUnimplemented(err error) bool {
	return IsError(12, err)
}

// Internal 出现内部服务器错误。通常是服务器错误
// HTTPCode 500
func Internal(reason, format string, a ...interface{}) error {
	return Errorf(13, reason, format, a...)
}

func IsInternal(err error) bool {
	return IsError(13, err)
}

// Unavailable 服务不可用。通常是服务器已关闭
// HTTPCode 503
func Unavailable(reason, format string, a ...interface{}) error {
	return Errorf(14, reason, format, a...)
}

func IsUnavailable(err error) bool {
	return IsError(14, err)
}

// DataLoss 出现不可恢复的数据丢失或数据损坏。客户端应该向用户报告错误
// HTTPCode 500
func DataLoss(reason, format string, a ...interface{}) error {
	return Errorf(15, reason, format, a...)
}

func IsDataLoss(err error) bool {
	return IsError(15, err)
}

// Unauthenticated 由于 OAuth 令牌丢失、无效或过期，请求未通过身份验证
// HTTPCode 401
func Unauthenticated(reason, format string, a ...interface{}) error {
	return Errorf(16, reason, format, a...)
}

func IsUnauthenticated(err error) bool {
	return IsError(16, err)
}
