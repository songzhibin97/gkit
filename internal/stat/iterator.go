package stat

import "fmt"

// Iterator 迭代窗口中所有桶
type Iterator struct {
	count         int
	iteratedCount int
	cur           *Bucket
}

// Next 返回 true 表示已经全部迭代完毕
func (i *Iterator) Next() bool {
	return i.count != i.iteratedCount
}

// Bucket 获取当前存储通
func (i *Iterator) Bucket() Bucket {
	if !(i.Next()) {
		panic(fmt.Errorf("stat/iterator: iteration out of range iteratedCount: %d count: %d", i.iteratedCount, i.count))
	}
	bucket := *i.cur
	i.iteratedCount++
	i.cur = i.cur.Next()
	return bucket
}
