package registry

import (
	"math/rand"
)

// subset google src 子集选择法

// Subset 子集选择法
// instances 实例列表
// size 选取的子集长度
func Subset(instances []interface{}, clientID int, size int) []interface{} {
	if len(instances) <= size {
		return instances
	}
	count := len(instances) / size
	// 将客户端划分为多轮,每一轮计算使用同样的随机排列的列表
	round := clientID / count
	s := rand.NewSource(int64(round))
	ra := rand.New(s)
	ra.Shuffle(len(instances), func(i, j int) {
		instances[i], instances[j] = instances[j], instances[i]
	})
	// clientID 代表目前的客户端
	start := (clientID % count) * size
	return instances[start : start+size]
}
