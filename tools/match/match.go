package match

import "unicode/utf8"

// 基础匹配器
// * 匹配任意数字字符
// ? 匹配任何一个字符

// Match 判断是否匹配
func Match(str, pattern string) bool {
	return deepMatch(str, pattern)
}

// deepMatch 深度匹配
func deepMatch(str, pattern string) bool {
	// 如果 pattern 只有 * 直接返回
	if pattern == "*" {
		return true
	}
	// pattern > 1 的情况 有连续 两个 ** 在一起进行删减 保证 *后面跟其他字符
	for len(pattern) > 1 && pattern[0] == '*' && pattern[1] == '*' {
		pattern = pattern[1:]
	}
	// 判断 pattern 长度是否有效
	for len(pattern) > 0 {
		if pattern[0] > 0x7f {
			// 0x7f ASCII 最后一位
			// 如果大于 0x7f 使用 utf-8 rune解析
			return deepMatchRune(str, pattern)
		}
		switch pattern[0] {
		default:
			// 如果 待匹配 str 为空 匹配失败
			if len(str) == 0 {
				return false
			}
			// 如果非ASCII进行rune解析
			if str[0] > 0x7f {
				return deepMatchRune(str, pattern)
			}
			// 判断str[0] 与 pattern[0] 是否相同
			if str[0] != pattern[0] {
				return false
			}
		case '?':
			// 如果是 ? 但是 str 为空则匹配失败
			if len(str) == 0 {
				return false
			}
		case '*':
			// 如果是 * 进行递归匹配
			return deepMatch(str, pattern[1:]) ||
				(len(str) > 0 && deepMatch(str[1:], pattern))
		}
		// str pattern
		str = str[1:]
		pattern = pattern[1:]
	}
	// 判断 str pattern 是否都匹配到
	return len(str) == 0 && len(pattern) == 0
}

func deepMatchRune(str, pattern string) bool {
	if pattern == "*" {
		return true
	}
	for len(pattern) > 1 && pattern[0] == '*' && pattern[1] == '*' {
		pattern = pattern[1:]
	}

	var sr, pr rune
	var srsz, prsz int

	if len(str) > 0 {
		// 判断如果是rune字节读取
		// 否则将ASCII转为rune
		if str[0] > 0x7f {
			sr, srsz = utf8.DecodeRuneInString(str)
		} else {
			sr, srsz = rune(str[0]), 1
		}
	} else {
		// 如果str为空设置为异常码
		sr, srsz = utf8.RuneError, 0
	}
	if len(pattern) > 0 {
		// str处理同理
		if pattern[0] > 0x7f {
			pr, prsz = utf8.DecodeRuneInString(pattern)
		} else {
			pr, prsz = rune(pattern[0]), 1
		}
	} else {
		pr, prsz = utf8.RuneError, 0
	}
	// done reading
	for pr != utf8.RuneError {
		switch pr {
		default:
			// 如果另外一个已经读空了 直接返回
			if srsz == utf8.RuneError {
				return false
			}
			// 判断两个字节是否相同
			if sr != pr {
				return false
			}
		case '?':
			// ? 如果另外一个已经读空了 直接返回
			if srsz == utf8.RuneError {
				return false
			}
		case '*':
			// * 递归匹配
			return deepMatchRune(str, pattern[prsz:]) ||
				(srsz > 0 && deepMatchRune(str[srsz:], pattern))
		}
		str = str[srsz:]
		pattern = pattern[prsz:]
		// 读取str pattern下一位
		if len(str) > 0 {
			if str[0] > 0x7f {
				sr, srsz = utf8.DecodeRuneInString(str)
			} else {
				sr, srsz = rune(str[0]), 1
			}
		} else {
			sr, srsz = utf8.RuneError, 0
		}
		if len(pattern) > 0 {
			if pattern[0] > 0x7f {
				pr, prsz = utf8.DecodeRuneInString(pattern)
			} else {
				pr, prsz = rune(pattern[0]), 1
			}
		} else {
			pr, prsz = utf8.RuneError, 0
		}
	}

	return srsz == 0 && prsz == 0
}

var maxRuneBytes = func() []byte {
	b := make([]byte, 4)
	if utf8.EncodeRune(b, '\U0010FFFF') != 4 {
		panic("invalid rune encoding")
	}
	return b
}()

// Allowable 返回符合条件的最大值和最小值
func Allowable(pattern string) (min, max string) {
	if pattern == "" || pattern[0] == '*' {
		return "", ""
	}

	minB := make([]byte, 0, len(pattern))
	maxB := make([]byte, 0, len(pattern))
	var wild bool
	for i := 0; i < len(pattern); i++ {
		if pattern[i] == '*' {
			wild = true
			break
		}
		if pattern[i] == '?' {
			minB = append(minB, 0)
			maxB = append(maxB, maxRuneBytes...)
		} else {
			minB = append(minB, pattern[i])
			maxB = append(maxB, pattern[i])
		}
	}
	if wild {
		r, n := utf8.DecodeLastRune(maxB)
		if r != utf8.RuneError {
			if r < utf8.MaxRune {
				r++
				if r > 0x7f {
					b := make([]byte, 4)
					nn := utf8.EncodeRune(b, r)
					maxB = append(maxB[:len(maxB)-n], b[:nn]...)
				} else {
					maxB = append(maxB[:len(maxB)-n], byte(r))
				}
			}
		}
	}
	return string(minB), string(maxB)
}

// IsPattern 判断是否含有 * ?
func IsPattern(str string) bool {
	for i := 0; i < len(str); i++ {
		if str[i] == '*' || str[i] == '?' {
			return true
		}
	}
	return false
}
