package stringx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIs(t *testing.T) {
	is := assert.New(t)

	is.False(IsNumeric(""))
	is.False(IsNumeric("  "))
	is.False(IsNumeric(" bob "))
	is.True(IsNumeric("123"))

	is.False(IsAlpha(""))
	is.False(IsAlpha(" "))
	is.False(IsAlpha(" Voa "))
	is.False(IsAlpha("123"))
	is.True(IsAlpha("Voa"))
	is.True(IsAlpha("bròwn"))

	is.False(IsAlphanumeric(""))
	is.False(IsAlphanumeric(" "))
	is.False(IsAlphanumeric(" Voa "))
	is.True(IsAlphanumeric("Voa"))
	is.True(IsAlphanumeric("123"))
	is.True(IsAlphanumeric("v123oa"))
	is.False(IsAlphanumeric("v123oa,"))
}
