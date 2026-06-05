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

func TestGetIndex_RejectsCrossResourceToken(t *testing.T) {
	// Both generators share the same salt (default) but identify different
	// resources. A token minted for "alpha" must not be accepted by "beta".
	a := NewTokenGenerate("alpha")
	b := NewTokenGenerate("beta")
	tok := a.ForIndex(7)
	if _, err := b.GetIndex(tok); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken on cross-resource token, got %v", err)
	}
}

func TestGetIndex_RejectsPrefixCollisionToken(t *testing.T) {
	// "user" is a string prefix of "user_admin". With the same (default) salt,
	// a token minted for the longer resource must NOT validate for the shorter
	// one, and vice versa. The bug: HasPrefix with no delimiter after the
	// resource id accepted "user_admin..." for "user".
	short := NewTokenGenerate("user")
	long := NewTokenGenerate("user_admin")

	if _, err := short.GetIndex(long.ForIndex(7)); err != ErrInvalidToken {
		t.Fatalf("user accepted a user_admin token: err = %v, want ErrInvalidToken", err)
	}
	if _, err := long.GetIndex(short.ForIndex(3)); err != ErrInvalidToken {
		t.Fatalf("user_admin accepted a user token: err = %v, want ErrInvalidToken", err)
	}
	// Sanity: each still round-trips its own token.
	if idx, err := short.GetIndex(short.ForIndex(5)); err != nil || idx != 5 {
		t.Fatalf("user self round-trip: idx=%d err=%v", idx, err)
	}
}

func TestNewTokenGenerateE_RequiresExplicitSalt(t *testing.T) {
	if _, err := NewTokenGenerateE("res"); err != ErrDefaultSalt {
		t.Fatalf("NewTokenGenerateE without SetSalt err = %v, want ErrDefaultSalt", err)
	}
	pt, err := NewTokenGenerateE("res", SetSalt("strong-salt"))
	if err != nil {
		t.Fatalf("NewTokenGenerateE with SetSalt err = %v", err)
	}
	idx, err := pt.GetIndex(pt.ForIndex(42))
	if err != nil {
		t.Fatalf("roundtrip err = %v", err)
	}
	if idx != 42 {
		t.Fatalf("idx = %d, want 42", idx)
	}
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
