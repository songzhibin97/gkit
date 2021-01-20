package stat

// Metric: 简单实现
// 度量标准软件包中度量标准的实现是:Counter, Gauge,PointGauge, RollingCounter and RollingGauge.
type Metric interface {
	// Add: 将给定值添加到当前窗口
	Add(int64)

	// Value: 获取当前值
	// 如果是 类型是 PointGauge, RollingCounter, RollingGauge
	// 返回窗口总和
	Value() int64
}

// Aggregation: 聚合接口
type Aggregation interface {
	// Min
	Min() float64

	// Max
	Max() float64

	// Avg
	Avg() float64

	// Sum
	Sum() float64
}
