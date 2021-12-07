package concurrent

import "reflect"

// FanInRec 扇入模式
func FanInRec(channels ...<-chan interface{}) <-chan interface{} {
	out := make(chan interface{}, 1)
	go func() {
		defer close(out)
		var cases []reflect.SelectCase
		for _, channel := range channels {
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(channel),
			})
		}
		for len(cases) > 0 {
			i, v, ok := reflect.Select(cases)
			if !ok {
				// 监控的channel已经关闭
				cases = append(cases[:i], cases[i+1:]...)
				continue
			}
			out <- v.Interface()
		}
	}()
	return out
}

// MergeChannel 合并两个channel
func MergeChannel(a, b <-chan interface{}) <-chan interface{} {
	c := make(chan interface{})
	go func() {
		defer close(c)
		for a != nil || b != nil {
			select {
			case v, ok := <-a:
				if !ok {
					a = nil
					continue
				}
				c <- v
			case v, ok := <-b:
				if !ok {
					b = nil
					continue
				}
				c <- v
			}
		}
	}()
	return c
}
