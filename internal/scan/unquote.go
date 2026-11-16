package scan

import (
	"encoding/binary"
	"math/bits"
	"slices"
)

const (
	surrogateHighMin = 0xD800
	surrogateHighMax = 0xDBFF
	surrogateLowMin  = 0xDC00
	surrogateLowMax  = 0xDFFF
	surrogateBase    = 0x10000
	surrogateShift   = 10
	simpleEscapeLen  = 2
	unicodeEscapeLen = 6
	hexDigitsLen     = 4
	invalidHex       = -1
	wordSize         = 8
	inlineRunWords   = 8
)

var hexValue = func() (t [256]int32) {
	for i := range t {
		t[i] = invalidHex
	}
	for c := '0'; c <= '9'; c++ {
		t[c] = c - '0'
	}
	for c := 'a'; c <= 'f'; c++ {
		t[c] = c - 'a' + 10
	}
	for c := 'A'; c <= 'F'; c++ {
		t[c] = c - 'A' + 10
	}
	return t
}()

var simpleEscapes = [256]byte{'"': '"', '\\': '\\', '/': '/', 'b': '\b', 'f': '\f', 'n': '\n', 'r': '\r', 't': '\t'}

func hex4(b []byte, i int) (rune, bool) {
	if i+hexDigitsLen > len(b) {
		return 0, false
	}
	v := hexValue[b[i]]<<12 | hexValue[b[i+1]]<<8 | hexValue[b[i+2]]<<4 | hexValue[b[i+3]]
	if hexValue[b[i]]|hexValue[b[i+1]]|hexValue[b[i+2]]|hexValue[b[i+3]] < 0 {
		return 0, false
	}
	return v, true
}

func AppendUnquoted(dst, b []byte, i int) ([]byte, int, bool) {
	for {
		var closed bool
		if dst, i, closed = unquoteFast(dst, b, i); closed {
			return dst, i, true
		}
		var j int
		dst, j = copySafeRun(dst, b, i)
		if j >= len(b) {
			return dst, j, false
		}
		if b[j] != '\\' {
			return dst, j, b[j] == '"'
		}
		if j+1 >= len(b) {
			return dst, j, false
		}
		if c := b[j+1]; c != 'u' {
			unescaped := simpleEscapes[c]
			if unescaped == 0 {
				return dst, j, false
			}
			dst = append(dst, unescaped)
			i = j + simpleEscapeLen
			continue
		}
		r, ok := hex4(b, j+2)
		if !ok {
			return dst, j, false
		}
		i = j + unicodeEscapeLen
		if r >= surrogateHighMin && r <= surrogateHighMax && i+1 < len(b) && b[i] == '\\' && b[i+1] == 'u' {
			if low, ok := hex4(b, i+2); ok && low >= surrogateLowMin && low <= surrogateLowMax {
				r = surrogateBase + (r-surrogateHighMin)<<surrogateShift + (low - surrogateLowMin)
				i += unicodeEscapeLen
			}
		}
		dst = AppendRune(dst, r)
	}
}

func copySafeRun(dst, b []byte, i int) ([]byte, int) {
	dst = slices.Grow(dst, inlineRunWords*wordSize)
	n := len(dst)
	buf := dst[:cap(dst)]
	for range inlineRunWords {
		if i+wordSize > len(b) {
			break
		}
		w := binary.LittleEndian.Uint64(b[i:])
		binary.LittleEndian.PutUint64(buf[n:], w)
		if m := SpecialMask(w); m != 0 {
			k := bits.TrailingZeros64(m) >> 3
			return buf[:n+k], i + k
		}
		i += wordSize
		n += wordSize
	}
	j := NextSpecial(b, i)
	return append(buf[:n], b[i:j]...), j
}

func AppendRune(dst []byte, r rune) []byte {
	switch {
	case r < 0x80:
		return append(dst, byte(r))
	case r < 0x800:
		return append(dst, byte(0xC0|r>>6), byte(0x80|r&0x3F))
	case r < 0x10000:
		return append(dst, byte(0xE0|r>>12), byte(0x80|(r>>6)&0x3F), byte(0x80|r&0x3F))
	default:
		return append(dst, byte(0xF0|r>>18), byte(0x80|(r>>12)&0x3F), byte(0x80|(r>>6)&0x3F), byte(0x80|r&0x3F))
	}
}
