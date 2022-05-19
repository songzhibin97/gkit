package stringx

import (
	"unicode"
)

// IsAlpha checks if the string contains only unicode letters.
func IsAlpha(s string) bool {
	if s == "" {
		return false
	}
	for _, v := range s {
		if !unicode.IsLetter(v) {
			return false
		}
	}
	return true
}

// IsAlphanumeric checks if the string contains only Unicode letters or digits.
func IsAlphanumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, v := range s {
		if !isAlphanumeric(v) {
			return false
		}
	}
	return true
}

// IsNumeric Checks if the string contains only digits. A decimal point is not a digit and returns false.
func IsNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, v := range s {
		if !unicode.IsDigit(v) {
			return false
		}
	}
	return true
}

func isAlphanumeric(v rune) bool {
	return unicode.IsDigit(v) || unicode.IsLetter(v)
}
