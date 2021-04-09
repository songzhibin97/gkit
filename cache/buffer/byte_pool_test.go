package buffer

import (
	"testing"
)

func Test_newBytePool(t *testing.T) {
	p := localBytePool
	t.Log(p)
}
