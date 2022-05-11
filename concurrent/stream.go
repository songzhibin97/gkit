package concurrent

import "context"

func Stream(ctx context.Context, values ...interface{}) <-chan interface{} {
	out := make(chan interface{})
	go func() {
		defer close(out)
		for _, value := range values {
			select {
			case <-ctx.Done():
				return
			case out <- value:
			}
		}
	}()
	return out
}

// TaskN 只取流中的前N个数据
func TaskN(ctx context.Context, valueStream <-chan interface{}, num int) <-chan interface{} {
	outStream := make(chan interface{})
	go func() {
		defer close(outStream)
		for i := 0; i < num; i++ {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-valueStream:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case outStream <- v:

				}
			}
		}
	}()
	return outStream
}

// TaskFn 筛选流中的数据,只保留满足条件的数据
func TaskFn(ctx context.Context, valueStream <-chan interface{}, fn func(v interface{}) bool) <-chan interface{} {
	outStream := make(chan interface{})
	go func() {
		defer close(outStream)
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-valueStream:
				if !ok {
					return
				}
				if fn(v) {
					select {
					case <-ctx.Done():
						return
					case outStream <- v:
					}
				}
			}
		}
	}()
	return outStream
}

// TaskWhile 只取满足条件的数据,一旦不满足就不再取
func TaskWhile(ctx context.Context, valueStream <-chan interface{}, fn func(v interface{}) bool) <-chan interface{} {
	outStream := make(chan interface{})
	go func() {
		defer close(outStream)
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-valueStream:
				if !ok {
					return
				}
				if fn(v) {
					select {
					case <-ctx.Done():
						return
					case outStream <- v:
					}
					return
				}
			}
		}
	}()
	return outStream
}

// SkipN 跳过流中的前N个数据
func SkipN(ctx context.Context, valueStream <-chan interface{}, num int) <-chan interface{} {
	outStream := make(chan interface{})
	go func() {
		defer close(outStream)
		for i := 0; i < num; i++ {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-valueStream:
				if !ok {
					return
				}
			}
		}
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-valueStream:
				if !ok {
					return
				}
				select {
				case <-ctx.Done():
					return
				case outStream <- v:
				}
			}
		}
	}()
	return outStream
}

// SkipFn 跳过满足条件的数据
func SkipFn(ctx context.Context, valueStream <-chan interface{}, fn func(v interface{}) bool) <-chan interface{} {
	outStream := make(chan interface{})
	go func() {
		defer close(outStream)
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-valueStream:
				if !ok {
					return
				}
				if !fn(v) {
					select {
					case <-ctx.Done():
						return
					case outStream <- v:
					}
				}
			}
		}
	}()
	return outStream
}

// SkipWhile 跳过满足条件的数据,一旦不满足,当前这个元素以后的元素都会输出
func SkipWhile(ctx context.Context, valueStream <-chan interface{}, fn func(v interface{}) bool) <-chan interface{} {
	outStream := make(chan interface{})
	go func() {
		defer close(outStream)
		for {
			select {
			case <-ctx.Done():
				return
			case v, ok := <-valueStream:
				if !ok {
					return
				}
				if fn(v) {
					select {
					case <-ctx.Done():
						return
					default:

					}
				} else {
					select {
					case <-ctx.Done():
						return
					case outStream <- v:
					}
					for {
						select {
						case <-ctx.Done():
							return
						case v, ok = <-valueStream:
							if !ok {
								return
							}
							select {
							case <-ctx.Done():
								return
							case outStream <- v:
							}
						}
					}
				}
			}
		}
	}()
	return outStream
}
