package vto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCompletionDefault(t *testing.T) {
	type (
		mock1 struct {
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

		mock2 struct {
			M1 mock1 `default:"{}"`
		}

		mock3 struct {
			M1 *mock1 `default:"{}"`
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
		var m1 mock1
		err := CompletionDefault(&m1)
		assert.NoError(t, err)
		assert.Equal(t, mock1{
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
		}, m1)
	}

	{
		var m2 mock2
		err := CompletionDefault(&m2)
		assert.NoError(t, err)
		assert.Equal(t, mock2{
			M1: mock1{
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
			},
		}, m2)
	}

	{
		var m3 mock3
		err := CompletionDefault(&m3)
		assert.NoError(t, err)
		assert.Equal(t, mock3{
			M1: &mock1{},
		}, m3)
	}
}
