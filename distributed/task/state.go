package task

type State int

const (
	// StatePending 任务初始状态
	StatePending State = 1 << iota
	// StateReceived 收到任务
	StateReceived
	// StateStarted 开始执行任务
	StateStarted
	// StateRetry 准备重试
	StateRetry
	// StateSuccess 任务成功
	StateSuccess
	// StateFailure 任务失败
	StateFailure
)

func (s State) String() string {
	switch s {
	case StatePending:
		return "PENDING"
	case StateReceived:
		return "RECEIVED"
	case StateStarted:
		return "STARTED"
	case StateRetry:
		return "RETRY"
	case StateSuccess:
		return "SUCCESS"
	case StateFailure:
		return "FAILURE"
	default:
		return "UNKNOWN"
	}
}
