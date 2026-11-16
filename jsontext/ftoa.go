package jsontext

import (
	"math"
	"slices"
	"strconv"

	"github.com/guno1928/alos-json/internal/fnum"
)

const (
	maxExactFloatInt   = 1 << 53
	maxTwoDecimalFloat = 1 << 46
	minPlainFloat      = 1e-6
	maxPlainFloat      = 1e21
	minPlainMagnitude  = -6
	maxPlainMagnitude  = 22
)

func usesExponentForm(f float64, bits int) bool {
	abs := math.Abs(f)
	if bits == 32 {
		abs32 := float32(abs)
		return abs32 < minPlainFloat || abs32 >= maxPlainFloat
	}
	return abs < minPlainFloat || abs >= maxPlainFloat
}

func appendExponentFloat(dst []byte, f float64, bits int) []byte {
	dst = strconv.AppendFloat(dst, f, 'e', -1, bits)
	if n := len(dst); n >= 4 && dst[n-4] == 'e' && dst[n-3] == '-' && dst[n-2] == '0' {
		dst[n-2] = dst[n-1]
		dst = dst[:n-1]
	}
	return dst
}

// FastAppendFloat appends the shortest decimal representation of f to dst.
// Uses fast paths for integers and common values, falling back to strconv
// for complex cases.
func FastAppendFloat(dst []byte, f float64, bits int) []byte {
	// Fast path: integer values (very common in JSON)
	if bits == 64 {
		if f == 0 {
			if math.Signbit(f) {
				return append(dst, '-', '0')
			}
			return append(dst, '0')
		}
		// Check if this is an integer value that fits in int64
		if f > -maxExactFloatInt && f < maxExactFloatInt {
			n := int64(f)
			if float64(n) == f {
				return appendInt64(dst, n)
			}
		}
	} else {
		// float32
		f32 := float32(f)
		if f32 == 0 {
			if math.Signbit(f) {
				return append(dst, '-', '0')
			}
			return append(dst, '0')
		}
		if f32 >= -1e7 && f32 <= 1e7 {
			n := int32(f32)
			if float32(n) == f32 {
				return appendInt64(dst, int64(n))
			}
		}
	}

	// Fast path: small decimals with 1-3 fractional digits (e.g., 98.5, 87.3)
	if bits == 64 && f > 0 && f < maxTwoDecimalFloat {
		if n := uint64(f*10 + 0.5); float64(n)/10 == f {
			dst = appendUint64(dst, n/10)
			return append(dst, '.', byte('0'+n%10))
		}
		// Check for 2 decimal places
		if n := uint64(f*100 + 0.5); float64(n)/100 == f {
			dst = appendUint64(dst, n/100)
			frac := n % 100
			return append(dst, '.', digitPairs[frac*2], digitPairs[frac*2+1])
		}
	}

	if bits == 64 {
		if digits, exp10, ok := fnum.Shortest64(f); ok {
			n := fnum.DecimalLen(digits)
			if magnitude := n + exp10; magnitude <= minPlainMagnitude || magnitude >= maxPlainMagnitude {
				return appendExponentFloat(dst, f, bits)
			}
			if f < 0 {
				dst = append(dst, '-')
			}
			return appendFixedDecimal(dst, digits, n, exp10)
		}
	}
	if usesExponentForm(f, bits) {
		return appendExponentFloat(dst, f, bits)
	}
	return strconv.AppendFloat(dst, f, 'f', -1, bits)
}

const zeroPointPrefix = "0."

func appendFixedDecimal(dst []byte, digits uint64, n, exp10 int) []byte {
	if exp10 >= 0 {
		return appendZeros(appendUint64(dst, digits), exp10)
	}
	point := n + exp10
	start := len(dst)
	if point > 0 {
		dst = slices.Grow(dst, n+1)[:start+n+1]
		out := dst[start:]
		fnum.PutDecimal(out[1:], digits)
		for i := 0; i < point; i++ {
			out[i] = out[i+1]
		}
		out[point] = '.'
		return dst
	}
	pad := len(zeroPointPrefix) - point
	dst = slices.Grow(dst, pad+n)[:start+pad+n]
	out := dst[start:]
	fnum.PutDecimal(out[pad:], digits)
	copy(out, zeroPointPrefix)
	for i := len(zeroPointPrefix); i < pad; i++ {
		out[i] = '0'
	}
	return dst
}

func appendZeros(dst []byte, n int) []byte {
	for range n {
		dst = append(dst, '0')
	}
	return dst
}
