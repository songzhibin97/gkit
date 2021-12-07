package concurrent

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func productionOrder() (ret []*OrderlyTask) {
	for i := 0; i < 10; i++ {
		i := i
		ret = append(ret, NewOrderTask(func() {
			fmt.Println(i)
		}))
	}
	return ret
}

func TestOrderly(t *testing.T) {
	var slice []int
	var productionOrder func() []*OrderlyTask = func() (ret []*OrderlyTask) {
		for i := 0; i < 10; i++ {
			i := i
			ret = append(ret, NewOrderTask(func() {
				slice = append(slice, i)
			}))
		}
		return ret
	}
	Orderly(productionOrder())
	assert.Equal(t, slice, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
}
