package concurrent

import "sync"

// FanOut 扇出模式
func FanOut(in <-chan interface{}, out []chan interface{}, async bool) {
	wg := sync.WaitGroup{}
	go func() {
		defer func() {
			wg.Wait()
			for i := 0; i < len(out); i++ {
				close(out[i])
			}
		}()
		for v := range in {
			for i := 0; i < len(out); i++ {
				if async {
					// 异步
					wg.Add(1)
					go func(ch chan interface{}, v interface{}) {
						defer wg.Done()
						ch <- v
					}(out[i], v)
				} else {
					// 同步
					out[i] <- v
				}
			}
		}
	}()
}
