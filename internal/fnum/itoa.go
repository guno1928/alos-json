package fnum

import (
	"encoding/binary"
	"math/bits"
	"slices"
)

const (
	eightDigitsMod   = 100_000_000
	sixteenDigitsMod = eightDigitsMod * eightDigitsMod
	pairBase         = 100
	pairTableMask    = 127
	wordBytes        = 8
	log10Of2Fixed    = 1233
	log10Of2FixShift = 12
)

var digitPair16 = func() (t [pairTableMask + 1]uint16) {
	for i := range pairBase {
		t[i] = uint16('0'+i/10) | uint16('0'+i%10)<<8
	}
	return t
}()

var pow10Uint64 = [...]uint64{
	1, 1e1, 1e2, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9, 1e10,
	1e11, 1e12, 1e13, 1e14, 1e15, 1e16, 1e17, 1e18, 1e19,
}

func DecimalLen(v uint64) int {
	t := bits.Len64(v|1) * log10Of2Fixed >> log10Of2FixShift
	if v|1 < pow10Uint64[t] {
		return t
	}
	return t + 1
}

const (
	fourDigitsMod  = 10_000
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

func appendExactlyEight(dst []byte, n uint32) []byte {
	start := len(dst)
	dst = slices.Grow(dst, wordBytes)[:start+wordBytes]
	binary.LittleEndian.PutUint64(dst[start:], eightDigitWord(n))
	return dst
}

func appendUpToEight(dst []byte, floor int, n uint32) []byte {
	if n < 10 {
		return append(dst, byte('0'+n))
	}
	if n < pairBase {
		pair := digitPair16[n]
		return append(dst, byte(pair), byte(pair>>8))
	}
	k := DecimalLen(uint64(n))
	drop := wordBytes - k
	w := eightDigitWord(n)
	start := len(dst)
	pos := start - drop
	if pos < floor {
		var tmp [wordBytes]byte
		binary.LittleEndian.PutUint64(tmp[:], w)
		return append(dst, tmp[drop:]...)
	}
	dst = slices.Grow(dst, k)[:start+k]
	keep := uint64(1)<<(8*drop) - 1
	prev := binary.LittleEndian.Uint64(dst[pos:])
	binary.LittleEndian.PutUint64(dst[pos:], prev&keep|w&^keep)
	return dst
}

func AppendUint64(dst []byte, floor int, v uint64) []byte {
	switch {
	case v < eightDigitsMod:
		return appendUpToEight(dst, floor, uint32(v))
	case v < sixteenDigitsMod:
		hi := v / eightDigitsMod
		dst = appendUpToEight(dst, floor, uint32(hi))
		return appendExactlyEight(dst, uint32(v-hi*eightDigitsMod))
	default:
		top := v / sixteenDigitsMod
		rest := v - top*sixteenDigitsMod
		mid := rest / eightDigitsMod
		dst = appendUpToEight(dst, floor, uint32(top))
		dst = appendExactlyEight(dst, uint32(mid))
		return appendExactlyEight(dst, uint32(rest-mid*eightDigitsMod))
	}
}

func AppendInt64(dst []byte, floor int, v int64) []byte {
	if v >= 0 {
		return AppendUint64(dst, floor, uint64(v))
	}
	return AppendUint64(append(dst, '-'), floor, -uint64(v))
}
