package concurrent

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func fanIn(start, end int) <-chan interface{} {
	out := make(chan interface{})
	go func() {
		defer close(out)
		for i := start; i < end; i++ {
			out <- i
		}
	}()
	return out
}

func TestFanInRec(t *testing.T) {
	out := FanInRec(fanIn(0, 6), fanIn(6, 11), fanIn(11, 20))
	outSlice := make([]interface{}, 0)
	for v := range out {
		outSlice = append(outSlice, v)
	}
	assert.Len(t, outSlice, 20)
	sort.Slice(outSlice, func(i, j int) bool {
		return outSlice[i].(int) < outSlice[j].(int)
	})
	assert.Equal(t, outSlice, []interface{}{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19})
}

func TestMergeChannel(t *testing.T) {
	out := MergeChannel(fanIn(0, 6), fanIn(6, 11))
	outSlice := make([]interface{}, 0)
	for v := range out {
		outSlice = append(outSlice, v)
	}
	assert.Len(t, outSlice, 11)
	sort.Slice(outSlice, func(i, j int) bool {
		return outSlice[i].(int) < outSlice[j].(int)
	})
	assert.Equal(t, outSlice, []interface{}{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})
}
