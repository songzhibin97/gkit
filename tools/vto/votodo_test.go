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
		mock4 struct {
			String     string   `default:"handsome"`
			Int8       int8     `default:"8"`
			Int16      int16    `default:"16"`
			Int32      int32    `default:"32"`
			Int64      int64    `default:"64"`
			Int        int      `default:"644"`
			Float32    float32  `default:"32.23"`
			Float64    float64  `default:"64.46"`
			Bool       bool     `default:"true"`
			StringPre  *string  `default:"handsome"`
			Int8Pre    *int8    `default:"8"`
			Int16Pre   *int16   `default:"16"`
			Int32Pre   *int32   `default:"32"`
			Int64Pre   *int64   `default:"64"`
			IntPre     *int     `default:"644"`
			Float32Pre *float32 `default:"32.23"`
			Float64Pre *float64 `default:"64.46"`
			BoolPre    *bool    `default:"true"`
		}
	)
	var (
		String  = "handsome"
		Int8    = int8(8)
		Int16   = int16(16)
		Int32   = int32(32)
		Int64   = int64(64)
		Int     = 644
		Float32 = float32(32.23)
		Float64 = 64.46
		Bool    = true
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

	{
		m0, m1 := mock4{}, mock4{}
		_ = VoToDo(&m0, &m1)
		m2 := mock4{
			String:     String,
			Int8:       Int8,
			Int16:      Int16,
			Int32:      Int32,
			Int64:      Int64,
			Int:        Int,
			Float32:    Float32,
			Float64:    Float64,
			Bool:       Bool,
			StringPre:  &String,
			Int8Pre:    &Int8,
			Int16Pre:   &Int16,
			Int32Pre:   &Int32,
			Int64Pre:   &Int64,
			IntPre:     &Int,
			Float32Pre: &Float32,
			Float64Pre: &Float64,
			BoolPre:    &Bool,
		}
		assert.Equal(t, m0, m2)
	}

}

