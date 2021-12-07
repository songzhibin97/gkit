package stat

// Bucket 桶对象
type Bucket struct {
	Points []float64
	Count  int64
	next   *Bucket
}

// Append 将给定值附加到存储桶中
func (b *Bucket) Append(val float64) {
	b.Points = append(b.Points, val)
	b.Count++
}

// Add 根据偏移量增加val
func (b *Bucket) Add(offset int, val float64) {
	b.Points[offset] += val
	b.Count++
}

// Reset 清空桶
func (b *Bucket) Reset() {
	b.Points = b.Points[:0]
	b.Count = 0
}

// Next 返回下一个存储桶
func (b *Bucket) Next() *Bucket {
	return b.next
}

// Window Window 对象
type Window struct {
	window []Bucket
	size   int
}

// NewWindow 实例化 Window 对象
func NewWindow(size int) *Window {
	buckets := make([]Bucket, size)
	for offset := range buckets {
		buckets[offset] = Bucket{Points: make([]float64, 0)}
		nextOffset := offset + 1
		if nextOffset == size {
			nextOffset = 0
		}
		buckets[offset].next = &buckets[nextOffset]
	}
	return &Window{window: buckets, size: size}
}

// ResetWindow 清空窗口中的所有存储桶。
func (w *Window) ResetWindow() {
	for offset := range w.window {
		w.ResetBucket(offset)
	}
}

// ResetBucket 根据给定的偏移量清空存储桶。
func (w *Window) ResetBucket(offset int) {
	w.window[offset].Reset()
}

// ResetBuckets 根据给定的偏移量清空存储桶。
func (w *Window) ResetBuckets(offsets []int) {
	for _, offset := range offsets {
		w.ResetBucket(offset)
	}
}

// Append 将给定值附加到索引等于给定偏移量的存储桶。
func (w *Window) Append(offset int, val float64) {
	w.window[offset].Append(val)
}

// Add 将给定值添加到存储桶中索引等于给定偏移量的最新点
func (w *Window) Add(offset int, val float64) {
	if w.window[offset].Count == 0 {
		w.window[offset].Append(val)
		return
	}
	w.window[offset].Add(0, val)
}

// Bucket 返回偏移量的窗口桶
func (w *Window) Bucket(offset int) Bucket {
	return w.window[offset]
}

// Size 返回窗口小
func (w *Window) Size() int {
	return w.size
}

// Iterator returns the bucket iterator.
func (w *Window) Iterator(offset int, count int) Iterator {
	return Iterator{
		count: count,
		cur:   &w.window[offset],
	}
}
