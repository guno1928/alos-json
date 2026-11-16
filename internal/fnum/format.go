package fnum

import (
	"math"
	"math/bits"
)

const (
	float64ExpOffset      = 1075
	float64Precision      = 53
	float64MantMask       = 1<<float64MantBits - 1
	float64CMin           = 1 << float64MantBits
	log10Of2Numerator     = 661_971_961_083
	log10Of3Over4Offset   = -274_743_187_321
	log10Shift            = 41
	log2Of10WideNumerator = 913_124_641_741
	log2Of10WideShift     = 38
	decimalBase           = 10
	dbKappa               = 2
	dbBigDivisor          = 1000
	dbSmallDivisor        = 100
	dbHalfSmallDivisor    = dbSmallDivisor / 2
	exactPow10Max128      = 55
	shorterTieExponent    = -77
	shorterLeftIntMin     = 2
	shorterLeftIntMax     = 3
	carrierTopBit         = 63
	carrierBits           = 64
	shorterLeftShift      = float64MantBits + 2
	shorterRightShift     = float64MantBits + 1
	shorterEndpointBase   = carrierBits - float64MantBits - 1
	shorterRoundBase      = carrierBits - float64MantBits - 2
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
	digits, exp10 = shortestNormal(b)
	for digits%decimalBase == 0 {
		digits /= decimalBase
		exp10++
	}
	return digits, exp10, true
}

func shortestNormal(b uint64) (uint64, int) {
	bq := int(b >> float64MantBits)
	t := b & float64MantMask
	mq := float64ExpOffset - bq
	c := float64CMin | t
	if mq > 0 && mq < float64Precision {
		if n := c >> uint(mq); n<<uint(mq) == c {
			return n, 0
		}
	}
	if t == 0 && bq > 1 {
		return shorterInterval(-mq)
	}
	return regularInterval(c, -mq)
}

func ceilPow10(k int) (uint64, uint64) {
	c := pow10Table[k-pow10Min]
	if uint(k) <= exactPow10Max128 {
		return c[0], c[1]
	}
	lo, carry := bits.Add64(c[1], 1, 0)
	return c[0] + carry, lo
}

func mulUpper(u, hi, lo uint64) (uint64, bool) {
	h1, l1 := bits.Mul64(u, hi)
	h2, _ := bits.Mul64(u, lo)
	mid, carry := bits.Add64(l1, h2, 0)
	return h1 + carry, mid == 0
}

func mulParity(twoF, hi, lo uint64, beta int) (bool, bool) {
	h2, l2 := bits.Mul64(twoF, lo)
	rHi := twoF*hi + h2
	parity := rHi>>uint(carrierBits-beta)&1 != 0
	return parity, rHi<<uint(beta)|l2>>uint(carrierBits-beta) == 0
}

func regularInterval(significand uint64, exponent int) (uint64, int) {
	twoFc := significand << 1
	even := significand&1 == 0
	minusK := floorLog10Pow2(exponent) - dbKappa
	hi, lo := ceilPow10(-minusK)
	beta := exponent + floorLog2Pow10Wide(-minusK)
	deltai := uint32(hi >> uint(carrierTopBit-beta))
	zi, zIsInteger := mulUpper((twoFc|1)<<uint(beta), hi, lo)
	sig := zi / dbBigDivisor
	r := uint32(zi - dbBigDivisor*sig)
	switch {
	case r < deltai:
		if r != 0 || !zIsInteger || even {
			return sig, minusK + dbKappa + 1
		}
		sig--
		r = dbBigDivisor
	case r == deltai:
		xParity, xIsInteger := mulParity(twoFc-1, hi, lo, beta)
		if xParity || xIsInteger && even {
			return sig, minusK + dbKappa + 1
		}
	}
	sig *= decimalBase
	dist := r - deltai/2 + dbHalfSmallDivisor
	approxYParity := (dist^dbHalfSmallDivisor)&1 != 0
	q := dist / dbSmallDivisor
	sig += uint64(q)
	if q*dbSmallDivisor == dist {
		yParity, yIsInteger := mulParity(twoFc, hi, lo, beta)
		if yParity != approxYParity || sig&1 != 0 && yIsInteger {
			sig--
		}
	}
	return sig, minusK + dbKappa
}

func shorterInterval(exponent int) (uint64, int) {
	minusK := floorLog10ThreeQuartersPow2(exponent)
	beta := exponent + floorLog2Pow10Wide(-minusK)
	hi, _ := ceilPow10(-minusK)
	xi := (hi - hi>>shorterLeftShift) >> uint(shorterEndpointBase-beta)
	zi := (hi + hi>>shorterRightShift) >> uint(shorterEndpointBase-beta)
	if exponent < shorterLeftIntMin || exponent > shorterLeftIntMax {
		xi++
	}
	if sig := zi / decimalBase; sig*decimalBase >= xi {
		return sig, minusK + 1
	}
	sig := (hi>>uint(shorterRoundBase-beta) + 1) / 2
	if sig&1 != 0 && exponent == shorterTieExponent {
		sig--
	} else if sig < xi {
		sig++
	}
	return sig, minusK
}
