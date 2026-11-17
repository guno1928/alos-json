package fnum

import (
	"encoding/binary"
	"math/bits"
	"slices"
)

const (
	eightDigitsMod   = 100_000_000
	pairBase         = 100
	wordBytes        = 8
	pairBytes        = 2
	log10Of2Fixed    = 1233
	log10Of2FixShift = 12
	leadFracBits     = 48
	leadFracMask     = 1<<leadFracBits - 1
	leadMulTwo       = 1<<leadFracBits/100 + 1
	leadMulFour      = 1<<leadFracBits/10_000 + 1
	leadMulSix       = 1<<leadFracBits/1_000_000 + 1
)

var pow10Uint64 = [...]uint64{
	1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10,
	1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
}

var digitPair16 = func() (t [pairBase]uint16) {
	for i := range t {
		t[i] = uint16('0'+i/10) | uint16('0'+i%10)<<8
	}
	return t
}()

var leadMul = [wordBytes + 1]uint64{0, 1 << leadFracBits, 1 << leadFracBits, leadMulTwo, leadMulTwo, leadMulFour, leadMulFour, leadMulSix, leadMulSix}

func DecimalLen(v uint64) int {
	t := bits.Len64(v|1) * log10Of2Fixed >> log10Of2FixShift
	if v|1 < pow10Uint64[t] {
		return t
	}
	return t + 1
}

const (
	threeDigitsMod = 1_000
	fourDigitsMod  = 10_000
	sixDigitsMod   = 1_000_000
	div100Mul      = 5243
	div100Shift    = 19
	div100LaneMask = 0x0000007F0000007F
	div10Mul       = 103
	div10Shift     = 10
	div10LaneMask  = 0x000F000F000F000F
	asciiZeroBytes = 0x3030303030303030
	pairLaneShift  = 16
	halfLaneShift  = 32
	digitByteShift = 8
)

func eightDigitWord(n uint32) uint64 {
	hi := n / fourDigitsMod
	x := uint64(hi) | uint64(n-hi*fourDigitsMod)<<halfLaneShift
	q := (x * div100Mul >> div100Shift) & div100LaneMask
	z := q | (x-q*pairBase)<<pairLaneShift
	t := (z * div10Mul >> div10Shift) & div10LaneMask
	return (t | (z-t*10)<<digitByteShift) + asciiZeroBytes
}

func putLead(b []byte, n uint32) {
	k := len(b)
	y := uint64(n) * leadMul[k]
	i := pairBytes
	if k&1 != 0 {
		b[0] = byte('0' + y>>leadFracBits)
		i = 1
	} else {
		binary.LittleEndian.PutUint16(b, digitPair16[y>>leadFracBits])
	}
	for ; i < k; i += pairBytes {
		y = (y & leadFracMask) * pairBase
		binary.LittleEndian.PutUint16(b[i:], digitPair16[y>>leadFracBits])
	}
}

func PutDecimal(b []byte, v uint64) {
	i := len(b)
	for i > wordBytes {
		q := v / eightDigitsMod
		binary.LittleEndian.PutUint64(b[i-wordBytes:], eightDigitWord(uint32(v-q*eightDigitsMod)))
		v = q
		i -= wordBytes
	}
	putLead(b[:i], uint32(v))
}

func AppendUint64(dst []byte, v uint64) []byte {
	if v < 10 {
		return append(dst, byte('0'+v))
	}
	if v < pairBase {
		pair := digitPair16[v]
		return append(dst, byte(pair), byte(pair>>digitByteShift))
	}
	if v < threeDigitsMod {
		q := v / pairBase
		low := digitPair16[v-q*pairBase]
		return append(dst, byte('0'+q), byte(low), byte(low>>digitByteShift))
	}
	if v < fourDigitsMod {
		q := v / pairBase
		high, low := digitPair16[q], digitPair16[v-q*pairBase]
		return append(dst, byte(high), byte(high>>digitByteShift), byte(low), byte(low>>digitByteShift))
	}
	if v < sixDigitsMod {
		top := v / fourDigitsMod
		rest := v - top*fourDigitsMod
		q := rest / pairBase
		mid, low := digitPair16[q], digitPair16[rest-q*pairBase]
		if top < 10 {
			return append(dst, byte('0'+top), byte(mid), byte(mid>>digitByteShift), byte(low), byte(low>>digitByteShift))
		}
		head := digitPair16[top]
		return append(dst, byte(head), byte(head>>digitByteShift), byte(mid), byte(mid>>digitByteShift), byte(low), byte(low>>digitByteShift))
	}
	k := DecimalLen(v)
	start := len(dst)
	dst = slices.Grow(dst, k)[:start+k]
	PutDecimal(dst[start:], v)
	return dst
}

func AppendInt64(dst []byte, v int64) []byte {
	u := uint64(v)
	if v < 0 {
		dst = append(dst, '-')
		u = -u
	}
	return AppendUint64(dst, u)
}
