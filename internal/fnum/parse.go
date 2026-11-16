package fnum

import (
	"encoding/binary"
	"math"
	"math/bits"
)

const (
	float64MantBits    = 52
	float64Bias        = -1023
	float64ExpMask     = 1<<11 - 1
	float64SignBit     = 1 << 63
	maxMantissaDigits  = 19
	maxExactMantissa   = 1 << 53
	maxExactPow10      = 22
	maxExponentDigits  = 3
	log2Of10Numerator  = 108853
	log2Of10Shift      = 15
	discardedBits      = 64 - float64MantBits - 3
	discardedBitsMask  = 1<<discardedBits - 1
	roundingCheckMask  = 3
	roundingHalfwayLow = 1
)

var exactPow10 = [...]float64{1e0, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10, 1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19, 1e20, 1e21, 1e22}

const (
	asciiZeros8      = 0x3030303030303030
	digitOverflow8   = 0x4646464646464646
	byteHighBits8    = 0x8080808080808080
	evenByteMask8    = 0x000000FF000000FF
	pairCombineMul   = 100 + 1_000_000<<32
	quadCombineMul   = 1 + 10_000<<32
	swarDigitWidth   = 8
	eightDigitsScale = 100_000_000
	asciiLowerBit    = 0x20
	byteBits         = 8
	scanWindow       = 32
	fastFractionMask = 15
)

func load8(s string, i int) uint64 {
	_ = s[i+7]
	return uint64(s[i]) | uint64(s[i+1])<<8 | uint64(s[i+2])<<16 | uint64(s[i+3])<<24 |
		uint64(s[i+4])<<32 | uint64(s[i+5])<<40 | uint64(s[i+6])<<48 | uint64(s[i+7])<<56
}

func nonDigitBytes(v uint64) uint64 {
	return ((v + digitOverflow8) | (v - asciiZeros8)) & byteHighBits8
}

func isEightDigits(v uint64) bool {
	return nonDigitBytes(v) == 0
}

func parseEightDigits(v uint64) uint64 {
	return combineEightDigits(v - asciiZeros8)
}

func combineEightDigits(v uint64) uint64 {
	v = v*10 + v>>8
	return (v&evenByteMask8*pairCombineMul + (v>>16)&evenByteMask8*quadCombineMul) >> 32
}

func ParseFloat64(s string) (float64, bool) {
	i := 0
	neg := false
	if i < len(s) && s[i] == '-' {
		neg = true
		i++
	}
	start := i
	var mant uint64
	for ; i < len(s) && s[i]-'0' < 10; i++ {
		mant = mant*10 + uint64(s[i]-'0')
	}
	if i == start {
		return 0, false
	}
	rawDigits := i - start
	exp10 := 0
	if i < len(s) && s[i] == '.' {
		i++
		fracStart := i
		for i+swarDigitWidth <= len(s) {
			v := load8(s, i)
			if !isEightDigits(v) {
				break
			}
			mant = mant*eightDigitsScale + parseEightDigits(v)
			i += swarDigitWidth
		}
		for ; i < len(s) && s[i]-'0' < 10; i++ {
			mant = mant*10 + uint64(s[i]-'0')
		}
		exp10 = fracStart - i
		rawDigits -= exp10
	}
	if rawDigits > maxMantissaDigits {
		var ok bool
		if mant, exp10, ok = significantMantissa(s[start:i]); !ok {
			return 0, false
		}
	}
	if i < len(s) {
		e, ok := parseExponent(s[i:])
		if !ok {
			return 0, false
		}
		exp10 += e
	}
	return decimalToFloat64(mant, exp10, neg)
}

func decimalToFloat64(mant uint64, exp10 int, neg bool) (float64, bool) {
	if mant <= maxExactMantissa && exp10 >= -maxExactPow10 && exp10 <= maxExactPow10 {
		f := float64(mant)
		if exp10 > 0 {
			f *= exactPow10[exp10]
		} else if exp10 < 0 {
			f /= exactPow10[-exp10]
		}
		if neg {
			f = -f
		}
		return f, true
	}
	return eiselLemire64(mant, exp10, neg)
}

func ScanFloat64(b []byte, i int) (float64, int, bool) {
	if i < 0 || len(b)-i < scanWindow {
		return scanFloat64Slow(b, i)
	}
	w := (*[scanWindow]byte)(b[i : i+scanWindow])
	neg := w[0] == '-'
	o := 0
	if neg {
		o = 1
	}
	v := binary.LittleEndian.Uint64(w[o:])
	k := uint(bits.TrailingZeros64(nonDigitBytes(v))) / byteBits
	if k-1 >= swarDigitWidth-1 || k > 1 && byte(v) == '0' {
		return scanFloat64Slow(b, i)
	}
	mant := partialDigits(v, k)
	pos := uint(o) + k
	exp10 := 0
	c := w[pos]
	if c == '.' {
		f := binary.LittleEndian.Uint64(w[pos+1:])
		if nd := nonDigitBytes(f); nd != 0 {
			fk := uint(bits.TrailingZeros64(nd)) / byteBits
			if fk == 0 {
				return scanFloat64Slow(b, i)
			}
			mant = mant*pow10Uint64[fk] + partialDigits(f, fk)
			pos += 1 + fk
			exp10 = -int(fk)
		} else {
			g := binary.LittleEndian.Uint64(w[pos+1+swarDigitWidth:])
			gk := uint(bits.TrailingZeros64(nonDigitBytes(g))) / byteBits
			if gk == swarDigitWidth || k+swarDigitWidth+gk > maxMantissaDigits {
				return scanFloat64Slow(b, i)
			}
			mant = (mant*eightDigitsScale+combineEightDigits(f-asciiZeros8))*pow10Uint64[gk] + partialDigits(g, gk)
			pos += 1 + swarDigitWidth + gk
			exp10 = -int(swarDigitWidth + gk)
		}
		c = w[pos]
	}
	if c|asciiLowerBit == 'e' {
		return scanFloat64Slow(b, i)
	}
	if mant > maxExactMantissa {
		f, ok := eiselLemire64(mant, exp10, neg)
		return f, i + int(pos), ok
	}
	f := float64(int64(mant)) / exactPow10[uint(-exp10)&fastFractionMask]
	if neg {
		f = -f
	}
	return f, i + int(pos), true
}

func partialDigits(v uint64, k uint) uint64 {
	return combineEightDigits((v - asciiZeros8) << (byteBits * (swarDigitWidth - k)))
}

func scanFloat64Slow(b []byte, i int) (float64, int, bool) {
	neg := i < len(b) && b[i] == '-'
	if neg {
		i++
	}
	if i >= len(b) {
		return 0, i, false
	}
	start := i
	var mant uint64
	if b[i] == '0' {
		i++
	} else if mant, i = accumulateDigits(b, i, 0); i == start {
		return 0, i, false
	}
	digits := i - start
	exp10 := 0
	if i < len(b) && b[i] == '.' {
		i++
		fracStart := i
		if mant, i = accumulateDigits(b, i, mant); i == fracStart {
			return 0, i, false
		}
		exp10 = fracStart - i
		digits -= exp10
	}
	if digits > maxMantissaDigits {
		return 0, i, false
	}
	if i < len(b) && b[i]|asciiLowerBit == 'e' {
		e, end, ok := scanExponent(b, i+1)
		if !ok {
			return 0, end, false
		}
		exp10 += e
		i = end
	}
	f, ok := decimalToFloat64(mant, exp10, neg)
	return f, i, ok
}

func accumulateDigits(b []byte, i int, mant uint64) (uint64, int) {
	for i+swarDigitWidth <= len(b) {
		v := binary.LittleEndian.Uint64(b[i:])
		if nonDigits := nonDigitBytes(v); nonDigits != 0 {
			n := uint(bits.TrailingZeros64(nonDigits)) / byteBits
			partial := (v - asciiZeros8) << (byteBits * (swarDigitWidth - n))
			return mant*pow10Uint64[n] + combineEightDigits(partial), i + int(n)
		}
		mant = mant*eightDigitsScale + combineEightDigits(v-asciiZeros8)
		i += swarDigitWidth
	}
	for ; i < len(b) && b[i]-'0' < 10; i++ {
		mant = mant*10 + uint64(b[i]-'0')
	}
	return mant, i
}

func scanExponent(b []byte, i int) (int, int, bool) {
	neg := false
	if i < len(b) && (b[i] == '+' || b[i] == '-') {
		neg = b[i] == '-'
		i++
	}
	start := i
	e := 0
	for ; i < len(b) && b[i]-'0' < 10; i++ {
		e = e*10 + int(b[i]-'0')
	}
	if i == start || i-start > maxExponentDigits {
		return 0, i, false
	}
	if neg {
		e = -e
	}
	return e, i, true
}

func significantMantissa(s string) (mant uint64, exp10 int, ok bool) {
	digits := 0
	inFraction := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '.' {
			inFraction = true
			continue
		}
		if mant != 0 || c != '0' {
			digits++
		}
		mant = mant*10 + uint64(c-'0')
		if inFraction {
			exp10--
		}
	}
	return mant, exp10, digits <= maxMantissaDigits
}

func parseExponent(s string) (int, bool) {
	if s[0] != 'e' && s[0] != 'E' {
		return 0, false
	}
	i := 1
	neg := false
	if i < len(s) && (s[i] == '+' || s[i] == '-') {
		neg = s[i] == '-'
		i++
	}
	if i == len(s) || len(s)-i > maxExponentDigits {
		return 0, false
	}
	e := 0
	for ; i < len(s); i++ {
		c := s[i] - '0'
		if c >= 10 {
			return 0, false
		}
		e = e*10 + int(c)
	}
	if neg {
		e = -e
	}
	return e, true
}

func floorLog2Pow10(e int) int {
	return (e * log2Of10Numerator) >> log2Of10Shift
}

func eiselLemire64(man uint64, exp10 int, neg bool) (float64, bool) {
	if man == 0 {
		if neg {
			return math.Copysign(0, -1), true
		}
		return 0, true
	}
	if exp10 < pow10Min || exp10 > pow10Max {
		return 0, false
	}
	pow := pow10Table[exp10-pow10Min]
	exp2 := 1 + floorLog2Pow10(exp10)
	clz := bits.LeadingZeros64(man)
	man <<= uint(clz)
	retExp2 := uint64(exp2+63-float64Bias) - uint64(clz)
	xHi, xLo := bits.Mul64(man, pow[0])
	if xHi&discardedBitsMask == discardedBitsMask && xLo+man < man {
		yHi, yLo := bits.Mul64(man, pow[1])
		mergedHi, mergedLo := xHi, xLo+yHi
		if mergedLo < xLo {
			mergedHi++
		}
		if mergedHi&discardedBitsMask == discardedBitsMask && mergedLo+1 == 0 && yLo+man < man {
			return 0, false
		}
		xHi, xLo = mergedHi, mergedLo
	}
	msb := xHi >> 63
	retMantissa := xHi >> (msb + discardedBits)
	retExp2 -= 1 ^ msb
	if xLo == 0 && xHi&discardedBitsMask == 0 && retMantissa&roundingCheckMask == roundingHalfwayLow {
		return 0, false
	}
	retMantissa += retMantissa & 1
	retMantissa >>= 1
	if retMantissa>>(float64MantBits+1) > 0 {
		retMantissa >>= 1
		retExp2++
	}
	if retExp2-1 >= float64ExpMask-1 {
		return 0, false
	}
	retBits := retExp2<<float64MantBits | retMantissa&(1<<float64MantBits-1)
	if neg {
		retBits |= float64SignBit
	}
	return math.Float64frombits(retBits), true
}
