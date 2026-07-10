package concurrent

import "reflect"

// OrDone returns a signal channel that closes when any non-nil input first
// becomes readable or closes. It never forwards an input value.
func OrDone(channels ...<-chan interface{}) <-chan interface{} {
	orDone := make(chan interface{})
	if len(channels) == 0 {
		close(orDone)
		return orDone
	}
	cases := make([]reflect.SelectCase, 0, len(channels))
	for _, channel := range channels {
		if channel == nil {
			continue
		}
		cases = append(cases, reflect.SelectCase{
			Dir:  reflect.SelectRecv,
			Chan: reflect.ValueOf(channel),
		})
	}
	if len(cases) == 0 {
		return orDone
	}

	go func() {
		defer close(orDone)
		reflect.Select(cases)
	}()
	return orDone
}
