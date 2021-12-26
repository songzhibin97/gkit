package deepcopy

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_deepCopy(t *testing.T) {
	type (
		mock struct {
			Name      string
			Age       int
			Money     float64
			Attribute map[string]string
			Friends   []string
		}
	)
	var (
		m1 = &mock{
			Name:      "gkit",
			Age:       21,
			Money:     10.01,
			Attribute: map[string]string{"job": "engineer"},
			Friends:   []string{"one", "two", "three"},
		}
		m2 = mock{
			Name:      "tikg",
			Age:       12,
			Money:     1.01,
			Attribute: map[string]string{"engineer": "job"},
			Friends:   []string{"three", "two", "one"},
		}
	)
	err := DeepCopy(m1, &m2)
	assert.NoError(t, err)
	assert.Equal(t, *m1, m2)
}
