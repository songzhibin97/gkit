package concurrent

import "reflect"

// OrDone 任意channel完成后返回
func OrDone(channels ...<-chan interface{}) <-chan interface{} {
	switch len(channels) {
	case 0:
		// 返回已经关闭的channel 通知各个接受者关闭
		c := make(chan interface{})
		close(c)
		return c
	case 1:
		return channels[0]
	}
	orDone := make(chan interface{}, 1)
	go func() {
		defer close(orDone)
		var cases []reflect.SelectCase
		for _, channel := range channels {
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectRecv,
				Chan: reflect.ValueOf(channel),
			})
		}
		// 选择一个可用的
		reflect.Select(cases)
	}()
	return orDone
}
