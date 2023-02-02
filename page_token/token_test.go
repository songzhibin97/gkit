package page_token

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewToken(t *testing.T) {
	n := NewTokenGenerate("test")

	i, err := n.GetIndex(n.ForIndex(1))
	assert.NoError(t, err)
	assert.Equal(t, i, 1)

	n2 := NewTokenGenerate("test", SetSalt("test1"))
	s2 := n2.ForIndex(1)
	_, err = n.GetIndex(s2)
	assert.Error(t, err)
	assert.Equal(t, err, ErrInvalidToken)

	n3 := NewTokenGenerate("test", SetMaxIndex(10))

	_, err = n3.GetIndex(n3.ForIndex(11))
	assert.Error(t, err)
	assert.Equal(t, err, ErrOverMaxPageSizeToken)

	n4 := NewTokenGenerate("test", SetTimeLimitation(time.Second*5))
	s4 := n4.ForIndex(1)
	time.Sleep(6 * time.Second)
	_, err = n4.GetIndex(s4)
	assert.Error(t, err)
	assert.Equal(t, err, ErrOverdueToken)
}

func Test_token_ProcessPageTokens(t *testing.T) {
	n := NewTokenGenerate("test")
	s, e, tk, err := n.ProcessPageTokens(10, 1, "")
	t.Log(s, e, tk)
	assert.NoError(t, err)
	for tk != "" {
		s, e, tk, err = n.ProcessPageTokens(10, 3, tk)
		t.Log(s, e, tk)
		assert.NoError(t, err)
	}
}
