package float

import "math"

// TruncFloat 截断float, prec 保留小数的位数
func TruncFloat(f float64, prec int) float64 {
	bit := math.Pow10(prec)
	return math.Trunc(f*bit) / bit
}
