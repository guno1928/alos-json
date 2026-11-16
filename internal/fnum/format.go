package fnum

import (
	"math"
	"math/bits"
)

const (
	float64QMin           = -1074
	float64ExpOffset      = 1075
	float64Precision      = 53
	float64MantMask       = 1<<float64MantBits - 1
	float64CMin           = 1 << float64MantBits
	log10Of2Numerator     = 661_971_961_083
	log10Of3Over4Offset   = -274_743_187_321
	log10Shift            = 41
	log2Of10WideNumerator = 913_124_641_741
	log2Of10WideShift     = 38
	schubfachTableShift   = 2
	mask63                = 1<<63 - 1
	decimalBase           = 10
)

func floorLog10Pow2(q int) int {
	return int(int64(q) * log10Of2Numerator >> log10Shift)
}

func floorLog10ThreeQuartersPow2(q int) int {
	return int((int64(q)*log10Of2Numerator + log10Of3Over4Offset) >> log10Shift)
}

func floorLog2Pow10Wide(e int) int {
	return int(int64(e) * log2Of10WideNumerator >> log2Of10WideShift)
}

func Shortest64(f float64) (digits uint64, exp10 int, ok bool) {
	b := math.Float64bits(f) &^ float64SignBit
	bq := int(b >> float64MantBits)
	t := b & float64MantMask
	if bq == float64ExpMask {
		return 0, 0, false
	}
	if bq == 0 {
		return 0, 0, t == 0
	}
	mq := float64ExpOffset - bq
	c := float64CMin | t
	if mq > 0 && mq < float64Precision {
		if n := c >> uint(mq); n<<uint(mq) == c {
			return n, 0, true
		}
	}
	digits, exp10 = schubfach(-mq, c)
	for digits%decimalBase == 0 {
		digits /= decimalBase
		exp10++
	}
	return digits, exp10, true
}

func schubfach(q int, c uint64) (uint64, int) {
	out := c & 1
	cb := c << 2
	cbr := cb + 2
	var cbl uint64
	var k int
	if c != float64CMin || q == float64QMin {
		cbl = cb - 2
		k = floorLog10Pow2(q)
	} else {
		cbl = cb - 1
		k = floorLog10ThreeQuartersPow2(q)
	}
	h := uint(q + floorLog2Pow10Wide(-k) + schubfachTableShift)
	g := schubfachG[k-schubfachKMin]
	vb := roundToOdd(g, cb<<h)
	vbl := roundToOdd(g, cbl<<h)
	vbr := roundToOdd(g, cbr<<h)
	s := vb >> 2
	sp10 := s / decimalBase * decimalBase
	tp10 := sp10 + decimalBase
	upin := vbl+out <= sp10<<2
	wpin := tp10<<2+out <= vbr
	if upin != wpin {
		if upin {
			return sp10, k
		}
		return tp10, k
	}
	t := s + 1
	uin := vbl+out <= s<<2
	win := t<<2+out <= vbr
	if uin != win {
		if uin {
			return s, k
		}
		return t, k
	}
	mid := (s + t) << 1
	if vb < mid || vb == mid && s&1 == 0 {
		return s, k
	}
	return t, k
}

func roundToOdd(g [2]uint64, cp uint64) uint64 {
	x1, _ := bits.Mul64(g[1], cp)
	y1, y0 := bits.Mul64(g[0], cp)
	z := y0>>1 + x1
	return y1 + z>>63 | (z&mask63+mask63)>>63
}
