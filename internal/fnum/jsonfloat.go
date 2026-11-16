package fnum

import (
	"encoding/binary"
	"math"
	"math/bits"
)

const (
	FloatWindow        = 32
	fixedWindow        = FloatWindow - 1
	fixedDigits        = 17
	maxExactFloat64    = 1 << 53
	maxCentsFloat64    = 1 << 46
	centsScale         = 100
	roundHalf          = 0.5
	minNormalBits      = 1 << float64MantBits
	infBits            = float64ExpMask << float64MantBits
	fifteenDigitsMin   = 1e14
	sixteenDigitsMin   = 1e15
	seventeenDigitsMin = 1e16
	seventeenDigitsEnd = 1e17
	hundredScale       = 100
	tenScale           = 10
	minPlainPoint      = -5
	maxPlainPoint      = 21
	plainPrefixWord    = 0x3030303030302e30
	plainPrefixLen     = 2
	byteMask           = 0xff
	lastByteShift      = 56
	byteIndexMask      = wordBytes - 1
	lowWordDigits      = wordBytes + 1
	pointedLen         = fixedDigits + 1
	quadSplitMul       = 1<<halfLaneShift - fourDigitsMod
	pairSplitMul       = 1<<pairLaneShift - pairBase
	digitSplitMul      = 1<<digitByteShift - decimalBase
	oneDigitExponent   = 10
	twoDigitExponent   = 100
	exponentMarkerLen  = 2
)

func PutFloat64(w *[FloatWindow]byte, f float64) int {
	fb := math.Float64bits(f)
	o := int(fb >> carrierTopBit)
	w[0] = '-'
	b := fb &^ float64SignBit
	if b-minNormalBits >= infBits-minNormalBits {
		if b != 0 {
			return 0
		}
		w[o] = '0'
		return o + 1
	}
	if a := math.Float64frombits(b); a < maxExactFloat64 {
		if u := int64(a); float64(u) == a {
			return o + putUint(w[o:], uint64(u))
		}
		if a < maxCentsFloat64 {
			if n := int64(a*centsScale + roundHalf); float64(n)/centsScale == a {
				return o + putCents(w[o:], uint64(n))
			}
		}
	}
	bq := int(b >> float64MantBits)
	t := b & float64MantMask
	var sig uint64
	var exp10 int
	if t == 0 && bq > 1 {
		sig, exp10 = shorterInterval(bq - float64ExpOffset)
	} else {
		sig, exp10 = regularInterval(float64CMin|t, bq-float64ExpOffset)
	}
	var point int
	if sig-fifteenDigitsMin < seventeenDigitsEnd-fifteenDigitsMin {
		scale := uint64(hundredScale)
		point = exp10 + fixedDigits - 2
		if sig >= sixteenDigitsMin {
			scale, point = tenScale, point+1
		}
		if sig >= seventeenDigitsMin {
			scale, point = 1, point+1
		}
		sig *= scale
	} else if sig, point = normalizeDigits(sig, exp10); sig == 0 {
		return 0
	}
	return o + putFixed17((*[fixedWindow]byte)(w[o:o+fixedWindow]), sig, point)
}

func normalizeDigits(sig uint64, exp10 int) (uint64, int) {
	n := DecimalLen(sig)
	if n > fixedDigits {
		return 0, 0
	}
	return sig * pow10Uint64[fixedDigits-n], exp10 + n
}

func putUint(d []byte, v uint64) int {
	if v >= pairBase {
		return putWideUint(d, v)
	}
	return putSmallUint(d, v)
}

func putSmallUint(d []byte, v uint64) int {
	pair, n := digitPair16[v%pairBase], pairBytes
	if v < decimalBase {
		pair, n = pair>>digitByteShift, 1
	}
	binary.LittleEndian.PutUint16(d, pair)
	return n
}

func putWideUint(d []byte, v uint64) int {
	k := DecimalLen(v)
	PutDecimal(d[:k], v)
	return k
}

func putCents(d []byte, n uint64) int {
	whole, cents := n/centsScale, n%centsScale
	var k int
	if whole < pairBase {
		k = putSmallUint(d, whole)
	} else {
		k = putWideUint(d, whole)
	}
	d[k] = '.'
	binary.LittleEndian.PutUint16(d[k+1:], digitPair16[cents])
	if cents%decimalBase == 0 {
		return k + pairBytes
	}
	return k + 1 + pairBytes
}

func digitsBigEndian(n uint32) uint64 {
	hi := n / fourDigitsMod
	x := uint64(n) + uint64(hi)*quadSplitMul
	q := (x * div100Mul >> div100Shift) & div100LaneMask
	z := x + q*pairSplitMul
	t := (z * div10Mul >> div10Shift) & div10LaneMask
	return z + t*digitSplitMul
}

func putFixed17(d *[fixedWindow]byte, sig uint64, point int) int {
	hi := sig / eightDigitsMod
	top := sig / seventeenDigitsMin
	loDigits := digitsBigEndian(uint32(sig - hi*eightDigitsMod))
	midDigits := digitsBigEndian(uint32(hi - top*eightDigitsMod))
	tz := bits.TrailingZeros64(loDigits) / digitByteShift
	if loDigits == 0 {
		tz += bits.TrailingZeros64(midDigits) / digitByteShift
	}
	lo := bits.ReverseBytes64(loDigits) + asciiZeroBytes
	mid := bits.ReverseBytes64(midDigits) + asciiZeroBytes
	lead := byte('0' + top)
	switch {
	case uint(point-1) < fixedDigits-1:
		d[0] = lead
		if q := uint(point - 1); q < wordBytes {
			binary.LittleEndian.PutUint64(d[1:], insertPoint(mid, q))
			binary.LittleEndian.PutUint64(d[lowWordDigits:], mid>>lastByteShift|lo<<digitByteShift)
		} else {
			binary.LittleEndian.PutUint64(d[1:], mid)
			binary.LittleEndian.PutUint64(d[lowWordDigits:], insertPoint(lo, q))
		}
		d[fixedDigits] = byte(lo >> lastByteShift)
		if tz < fixedDigits-point {
			return pointedLen - tz
		}
		return point
	case uint(point-fixedDigits) <= maxPlainPoint-fixedDigits:
		d[0] = lead
		binary.LittleEndian.PutUint64(d[1:], mid)
		binary.LittleEndian.PutUint64(d[lowWordDigits:], lo)
		binary.LittleEndian.PutUint64(d[fixedDigits:], asciiZeroBytes)
		return point
	case uint(point-minPlainPoint) <= -minPlainPoint:
		binary.LittleEndian.PutUint64(d[0:], plainPrefixWord)
		s := uint(plainPrefixLen-point) & byteIndexMask
		d[s] = lead
		binary.LittleEndian.PutUint64(d[s+1:], mid)
		binary.LittleEndian.PutUint64(d[s+lowWordDigits:], lo)
		return int(s) + fixedDigits - tz
	}
	return putExponentForm(d, lead, mid, lo, fixedDigits-tz, point-1)
}

func insertPoint(x uint64, q uint) uint64 {
	shift := (q & byteIndexMask) * digitByteShift
	low := uint64(1)<<shift - 1
	return x&low | uint64('.')<<shift | (x<<digitByteShift)&^(low<<digitByteShift|byteMask)
}

func putExponentForm(d *[fixedWindow]byte, lead byte, mid, lo uint64, digits, exp int) int {
	d[0] = lead
	d[1] = '.'
	binary.LittleEndian.PutUint64(d[2:], mid)
	binary.LittleEndian.PutUint64(d[2+wordBytes:], lo)
	e := digits + 1
	if digits == 1 {
		e = 1
	}
	d[e] = 'e'
	d[e+1] = '+'
	if exp < 0 {
		d[e+1] = '-'
		exp = -exp
	}
	e += exponentMarkerLen
	switch {
	case exp < oneDigitExponent:
		d[e] = byte('0' + exp)
		return e + 1
	case exp < twoDigitExponent:
		binary.LittleEndian.PutUint16(d[e:], digitPair16[exp])
		return e + pairBytes
	}
	d[e] = byte('0' + exp/twoDigitExponent)
	binary.LittleEndian.PutUint16(d[e+1:], digitPair16[exp%twoDigitExponent])
	return e + 1 + pairBytes
}
