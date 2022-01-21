package task

// Processor 任务处理器
type Processor interface {
	// Process 处理程序
	Process(t *Signature) error
	// ConsumeQueue 消费队列
	ConsumeQueue() string
	// PreConsumeHandler 是否进行预处理
	PreConsumeHandler() bool
}
