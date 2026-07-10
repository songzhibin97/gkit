package registry

import (
	"math/rand"
)

// subset google src 子集选择法

// Subset 子集选择法
// instances 实例列表
// size 选取的子集长度
func Subset(instances []interface{}, clientID int, size int) []interface{} {
	if size <= 0 {
		return nil
	}
	if len(instances) <= size {
		return instances
	}
	cp := make([]interface{}, len(instances))
	copy(cp, instances)
	count := len(cp) / size
	round := clientID / count
	s := rand.NewSource(int64(round))
	ra := rand.New(s)
	ra.Shuffle(len(cp), func(i, j int) {
		cp[i], cp[j] = cp[j], cp[i]
	})
	// clientID 代表目前的客户端
	start := (clientID % count) * size
	return cp[start : start+size]
}
