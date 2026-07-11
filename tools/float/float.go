package float

import "math"

// TruncFloat 截断float, prec 保留小数的位数
func TruncFloat(f float64, prec int) float64 {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return f
	}

	scale := math.Pow10(prec)
	if scale == 0 {
		return math.Copysign(0, f)
	}
	if math.IsInf(scale, 1) {
		return f
	}

	scaled := f * scale
	if math.IsInf(scaled, 0) {
		return f
	}
	return math.Trunc(scaled) / scale
}
