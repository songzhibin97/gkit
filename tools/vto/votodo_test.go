package vto

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestVoToDo(t *testing.T) {

	type (
		mock1 struct {
			Name string
			Age  int
		}
		mock2 struct {
			Mock *mock1
			Ok   bool
		}
		mock3 struct {
			Mock *mock1
			Ok   *bool
		}
	)
	{
		m0, m1 := mock1{}, mock1{
			Name: "gkit",
			Age:  1,
		}
		err := VoToDo(m0, &m1)
		assert.Error(t, err, "dst 或 src 必须是指针类型")
		err = VoToDo(&m0, m1)
		assert.Error(t, err, "dst 或 src 必须是指针类型")
	}

	{
		m0, m1 := mock1{}, mock1{
			Name: "gkit",
			Age:  1,
		}
		_ = VoToDo(&m0, &m1)
		assert.Equal(t, m0, m1)
	}

	{

		m0, m1 := mock2{}, mock2{
			Mock: &mock1{
				Name: "gkit",
				Age:  2,
			},
			Ok: true,
		}
		_ = VoToDo(&m0, &m1)
		assert.Equal(t, m0, m1)
	}
	{
		tt := true
		m0, m1 := mock2{}, mock3{
			Mock: &mock1{
				Name: "gkit",
				Age:  3,
			},
			Ok: &tt,
		}
		_ = VoToDo(&m0, &m1)
		assert.Equal(t, m0.Mock, m1.Mock)
		assert.Equal(t, m0.Ok, *m1.Ok)
	}
}
