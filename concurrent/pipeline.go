package concurrent

// Pipeline 串联模式
func Pipeline(in chan interface{}) <-chan interface{} {
	out := make(chan interface{},1)
	go func() {
		for v := range in {
			out <- v
		}
	}()
	return out
}
