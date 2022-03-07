package log

type Lever int8

// 预定义Level等级
const (
	LevelDebug Lever = iota
	LevelInfo
	LevelWarn
	LevelError
)

var m = map[Lever]string{
	LevelDebug: "[Debug]",
	LevelInfo:  "[Info]",
	LevelWarn:  "[Warn]",
	LevelError: "[Error]",
}

// Allow 允许是否可以打印
func (l Lever) Allow(lv Lever) bool {
	return lv >= l
}

// String 语义转义
func (l Lever) String() string {
	if v, ok := m[l]; ok {
		return v
	}
	return "UNKNOWN"
}
