package gctuner

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMem(t *testing.T) {
	is := assert.New(t)
	const mb = 1024 * 1024

	heap := make([]byte, 100*mb+1)
	inuse := readMemoryInuse()
	t.Logf("mem inuse: %d MB", inuse/mb)
	is.GreaterOrEqual(inuse, uint64(100*mb))
	heap[0] = 0
}
