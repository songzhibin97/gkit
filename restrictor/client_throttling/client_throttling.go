package client_throttling

import "github.com/songzhibin97/gkit/tools/float"

// 客户端节流算法
// google sre p330

var DefaultK = 2.0

// RejectionProbability 客户端节流算法
// requests 请求数量 应用层代码发出的所有请求的数量总计（指运行于自适应节流系统之上的应用代码
// accepts 请求接受数量 后端任务接受的请求数量
// k 倍值 降低倍值会使自适应节流算法更加激进
// 举例来说,假设将客户端请求的上限从request=2 * accepts调整为request=1.1* accepts,那么就意味着每10个后端请求之中只有1个会被拒绝
func RejectionProbability(requests int, accepts int, k float64) float64 {
	return max(0, float.TruncFloat((float64(requests)-k*float64(accepts))/float64(requests+1), 2))
}

func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
