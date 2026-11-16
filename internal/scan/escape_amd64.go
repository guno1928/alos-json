//go:build amd64 && !purego

package scan

import "math/bits"

const (
	evenBits       = 0x5555555555555555
	unicodeHexRun  = 4
	unicodeCarryAt = BlockLen - unicodeHexRun
)

type escapeBlock struct {
	quote     uint64
	bslash    uint64
	ctrl      uint64
	notSimple uint64
	notU      uint64
	notHex    uint64
}

//go:noescape
func escapeMasksAVX2(p *byte, out *escapeBlock)

func escapedMask(backslash uint64, prevEscaped *uint64) uint64 {
	backslash &^= *prevEscaped
	follows := backslash<<1 | *prevEscaped
	oddStarts := backslash &^ evenBits &^ follows
	seq, carry := bits.Add64(oddStarts, backslash, 0)
	*prevEscaped = carry
	return (evenBits ^ seq<<1) & follows
}

func escapedStringEnd(b []byte, pos *int) (end int, done, ok bool) {
	if !useAVX2 {
		return escapedStringEndScalar(b, pos)
	}
	if end, ok = escapedStringBlocks(b, *pos); !ok {
		end, ok = scalarStringEnd(b, *pos)
	}
	return end, true, ok
}

func escapedStringBlocks(b []byte, i int) (int, bool) {
	var prevEscaped, pendingHex uint64
	var pad [BlockLen]byte
	var m escapeBlock
	for {
		if i+BlockLen <= len(b) {
			escapeMasksAVX2(&b[i], &m)
		} else {
			copy(pad[:], b[i:])
			escapeMasksAVX2(&pad[0], &m)
		}
		escaped := escapedMask(m.bslash, &prevEscaped)
		unicode := escaped &^ m.notU
		required := pendingHex | unicode<<1 | unicode<<2 | unicode<<3 | unicode<<4
		pendingHex = unicode>>(BlockLen-1) | unicode>>(BlockLen-2) | unicode>>(BlockLen-3) | unicode>>unicodeCarryAt
		quote := m.quote &^ escaped
		upto := ^uint64(0)
		if quote != 0 {
			upto = quote ^ (quote - 1)
		}
		if bad := (m.ctrl | escaped&m.notSimple&m.notU | required&m.notHex) & upto; bad != 0 {
			return i + bits.TrailingZeros64(bad), false
		}
		if quote != 0 {
			return i + bits.TrailingZeros64(quote), true
		}
		i += BlockLen
	}
}

func scalarStringEnd(b []byte, i int) (int, bool) {
	for {
		i = NextSpecial(b, i)
		if i >= len(b) {
			return i, false
		}
		switch b[i] {
		case '"':
			return i, true
		case '\\':
			n := escapeLen(b, i)
			if n == 0 {
				return i, false
			}
			i += n
		default:
			return i, false
		}
	}
}
