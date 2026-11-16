package scan

import "encoding/binary"

const (
	BlockLen          = 64
	highBitGather     = 0x0102040810204080
	gatheredByteShift = 56
)

const (
	low7Bits8 = 0x7F7F7F7F7F7F7F7F
	ctrlBits8 = 0xE0E0E0E0E0E0E0E0
)

func exactZeroBytes(v uint64) uint64 {
	return ^(((v & low7Bits8) + low7Bits8) | v) & swarHighs
}

func exactSpecialMask(w uint64) uint64 {
	return exactZeroBytes(w^swarQuote) | exactZeroBytes(w^swarSlash) | exactZeroBytes(w&ctrlBits8)
}

func compressHighBits(m uint64) uint64 {
	return (m >> 7) * highBitGather >> gatheredByteShift
}

func specialMask64Generic(b []byte) uint64 {
	_ = b[BlockLen-1]
	var mask uint64
	for w := range BlockLen / 8 {
		mask |= compressHighBits(exactSpecialMask(binary.LittleEndian.Uint64(b[w*8:]))) << (w * 8)
	}
	return mask
}
