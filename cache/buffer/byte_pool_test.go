/******
** @创建时间 : 2021/2/20 15:05
** @作者 : SongZhiBin
******/
package buffer

import (
	"testing"
)

func Test_newBytePool(t *testing.T) {
	p := localBytePool
	t.Log(p)
}
