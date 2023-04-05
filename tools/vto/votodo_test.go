package vto

import (
	"testing"

	"github.com/songzhibin97/gkit/tools"

	"github.com/stretchr/testify/assert"
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
		assert.Error(t, err, tools.ErrorMustPtr)
		err = VoToDo(&m0, m1)
		assert.Error(t, err, tools.ErrorMustPtr)
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
	{
		m0, m1 := 1, mock4{}
		err := VoToDo(&m0, &m1)
		assert.Error(t, err, tools.ErrorMustStructPtr)
	}
}

func TestVoToDoRecursion(t *testing.T) {
	type (
		mock1 struct {
			Name string `json:"name"`
		}

		mock2 struct {
			Name string `json:"name"`
		}

		mock3 struct {
			Name string `json:"name"`
			Mock mock1  `json:"mock"`
		}

		mock4 struct {
			Name string `json:"name"`
			Mock mock2  `json:"mock"`
		}
	)

	var (
		m1 = mock3{
			Name: "m1",
			Mock: mock1{
				Name: "m1.mock1",
			},
		}
	)
	{
		var m2 mock4
		err := VoToDo(&m2, &m1)
		assert.NoError(t, err)
		assert.Equal(t, m2.Name, m1.Name)
		assert.Equal(t, m2.Mock.Name, m1.Mock.Name)
	}
}

func TestVoToDoPlus(t *testing.T) {
	type (
		mock1 struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		mock2 struct {
			Name1 string `json:"name"`
			Age   int    `json:"age1"`
		}
		mock3 struct {
			Name string `json:"name1"`
			Age  int    `json:"age"`
		}
		mock4 struct {
			Name1 string `json:"name1"`
			Name  string `json:"name"`
		}
		mock5 struct {
			Name string `gkit:"name1"`
			Age  int    `gkit:"age"`
		}
		mock6 struct {
			Name1 string `gkit:"name1"`
			Name  string `gkit:"name"`
		}
		mock7 struct {
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
		m0, m1 := mock1{}, mock2{
			Name1: "gkit",
			Age:   1,
		}
		err := VoToDoPlus(m0, &m1, ModelParameters{})
		assert.Error(t, err, tools.ErrorMustPtr)
		err = VoToDoPlus(&m0, m1, ModelParameters{})
		assert.Error(t, err, tools.ErrorMustPtr)
	}

	{
		m0, m1 := mock1{}, mock2{
			Name1: "gkit",
			Age:   1,
		}
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: FieldBind,
		})
		assert.Equal(t, m0.Age, m1.Age)
		assert.NotEqual(t, m0.Name, m1.Name1)
	}
	{
		m0, m1 := mock1{}, mock2{
			Name1: "gkit",
			Age:   1,
		}
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: TagBind,
		})
		assert.Equal(t, m0.Name, m1.Name1)
		assert.NotEqual(t, m0.Age, m1.Age)
	}
	{
		m0, m1 := mock1{}, mock2{
			Name1: "gkit",
			Age:   1,
		}
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: FieldBind | TagBind | OverlayBind,
		})
		assert.Equal(t, m0.Name, m1.Name1)
		assert.Equal(t, m0.Age, m1.Age)
	}
	{
		m0, m1 := mock3{}, mock4{
			Name1: "gkit",
			Name:  "gkit1",
		}
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: FieldBind,
		})
		assert.Equal(t, m0.Name, m1.Name)
		assert.NotEqual(t, m0.Name, m1.Name1)
	}
	{
		m0, m1 := mock3{}, mock4{
			Name1: "gkit",
			Name:  "gkit1",
		}
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: TagBind,
		})
		assert.Equal(t, m0.Name, m1.Name1)
		assert.NotEqual(t, m0.Name, m1.Name)
	}
	{
		m0, m1 := mock3{}, mock4{
			Name1: "gkit",
			Name:  "gkit1",
		}
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: FieldBind | TagBind,
		})
		assert.Equal(t, m0.Name, m1.Name)
		assert.NotEqual(t, m0.Name, m1.Name1)
	}
	{
		m0, m1 := mock3{}, mock4{
			Name1: "gkit",
			Name:  "gkit1",
		}
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: FieldBind | TagBind | OverlayBind,
		})
		assert.Equal(t, m0.Name, m1.Name1)
		assert.NotEqual(t, m0.Name, m1.Name)
	}

	{
		m0, m1 := mock5{}, mock6{
			Name1: "gkit",
			Name:  "gkit1",
		}
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: TagBind,
			Tag:   "gkit",
		})
		assert.Equal(t, m0.Name, m1.Name1)
		assert.NotEqual(t, m0.Name, m1.Name)
	}
	{
		m0, m1 := mock5{}, mock6{
			Name1: "gkit",
			Name:  "gkit1",
		}
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: FieldBind | TagBind,
			Tag:   "gkit",
		})
		assert.Equal(t, m0.Name, m1.Name)
		assert.NotEqual(t, m0.Name, m1.Name1)
	}
	{
		m0, m1 := mock5{}, mock6{
			Name1: "gkit",
			Name:  "gkit1",
		}
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: FieldBind | TagBind | OverlayBind,
			Tag:   "gkit",
		})
		assert.Equal(t, m0.Name, m1.Name1)
		assert.NotEqual(t, m0.Name, m1.Name)
	}
	{
		m0, m1 := mock5{}, mock6{
			Name1: "gkit",
			Name:  "gkit1",
		}
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: FieldBind | TagBind | OverlayBind,
			Tag:   "gkit",
		})
		assert.Equal(t, m0.Name, m1.Name1)
		assert.NotEqual(t, m0.Name, m1.Name)
	}

	{
		m0, m1 := mock7{}, mock7{
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
		_ = VoToDoPlus(&m0, &m1, ModelParameters{
			Model: DefaultValueBind,
		})
		assert.Equal(t, m0, m1)

	}
}
