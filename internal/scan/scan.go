package scan

import (
	"encoding/binary"
	"math/bits"
)

const (
	MaxDepth = 10000

	swarOnes  = 0x0101010101010101
	swarHighs = 0x8080808080808080
	swarQuote = 0x2222222222222222
	swarSlash = 0x5C5C5C5C5C5C5C5C
	swarCtrl  = 0x2020202020202020
)

var isSpace = [256]bool{' ': true, '\t': true, '\n': true, '\r': true}

var isHex = func() (t [256]bool) {
	for c := '0'; c <= '9'; c++ {
		t[c] = true
	}
	for c := 'a'; c <= 'f'; c++ {
		t[c] = true
	}
	for c := 'A'; c <= 'F'; c++ {
		t[c] = true
	}
	return t
}()

func IsSpace(c byte) bool { return isSpace[c] }

func SkipSpace(b []byte, i int) int {
	for i < len(b) && isSpace[b[i]] {
		i++
	}
	return i
}

func SpecialMask(w uint64) uint64 {
	ctrl := (w - swarCtrl) &^ w
	q := w ^ swarQuote
	quote := (q - swarOnes) &^ q
	s := w ^ swarSlash
	slash := (s - swarOnes) &^ s
	return (ctrl | quote | slash) & swarHighs
}

const (
	swarPrefixWords = 4
	kernelMinLen    = 32
)

func NextSpecial(b []byte, i int) int {
	for range swarPrefixWords {
		if i+8 > len(b) {
			break
		}
		if m := SpecialMask(binary.LittleEndian.Uint64(b[i:])); m != 0 {
			return i + bits.TrailingZeros64(m)>>3
		}
		i += 8
	}
	if len(b)-i >= kernelMinLen {
		return i + findSafe(&b[i], len(b)-i)
	}
	for i+8 <= len(b) {
		if m := SpecialMask(binary.LittleEndian.Uint64(b[i:])); m != 0 {
			return i + bits.TrailingZeros64(m)>>3
		}
		i += 8
	}
	for i < len(b) {
		c := b[i]
		if c < 0x20 || c == '"' || c == '\\' {
			return i
		}
		i++
	}
	return i
}

func StringEnd(b []byte, i int) (end int, escaped bool, ok bool) {
	for {
		i = NextSpecial(b, i)
		if i >= len(b) {
			return i, escaped, false
		}
		switch b[i] {
		case '"':
			return i, escaped, true
		case '\\':
			escaped = true
			n := escapeLen(b, i)
			if n == 0 {
				return i, escaped, false
			}
			i += n
			if i+BlockLen <= len(b) {
				if end, done, ok := escapedStringEnd(b, &i); done {
					return end, true, ok
				}
			}
		default:
			return i, escaped, false
		}
	}
}

func escapeLen(b []byte, i int) int {
	if i+1 >= len(b) {
		return 0
	}
	switch b[i+1] {
	case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
		return 2
	case 'u':
		if i+5 >= len(b) || !isHex[b[i+2]] || !isHex[b[i+3]] || !isHex[b[i+4]] || !isHex[b[i+5]] {
			return 0
		}
		return 6
	}
	return 0
}

func escapedStringEndScalar(b []byte, pos *int) (end int, done, ok bool) {
	i := *pos
	for i+BlockLen <= len(b) {
		m := SpecialMask64(b[i : i+BlockLen])
		advance := BlockLen
		for m != 0 {
			off := bits.TrailingZeros64(m)
			p := i + off
			switch b[p] {
			case '"':
				return p, true, true
			case '\\':
				n := escapeLen(b, p)
				if n == 0 {
					return p, true, false
				}
				if off+n >= BlockLen {
					advance = off + n
					m = 0
					continue
				}
				m &^= 1<<uint(off+n) - 1
			default:
				return p, true, false
			}
		}
		i += advance
	}
	*pos = i
	return 0, false, false
}

func isDigit(c byte) bool { return c-'0' < 10 }

func NumberEnd(b []byte, i int) (int, bool) {
	if i < len(b) && b[i] == '-' {
		i++
	}
	if i >= len(b) {
		return i, false
	}
	switch {
	case b[i] == '0':
		i++
	case b[i] >= '1' && b[i] <= '9':
		i++
		for i < len(b) && isDigit(b[i]) {
			i++
		}
	default:
		return i, false
	}
	if i < len(b) && b[i] == '.' {
		i++
		if i >= len(b) || !isDigit(b[i]) {
			return i, false
		}
		for i < len(b) && isDigit(b[i]) {
			i++
		}
	}
	if i < len(b) && (b[i] == 'e' || b[i] == 'E') {
		i++
		if i < len(b) && (b[i] == '+' || b[i] == '-') {
			i++
		}
		if i >= len(b) || !isDigit(b[i]) {
			return i, false
		}
		for i < len(b) && isDigit(b[i]) {
			i++
		}
	}
	return i, true
}

func LiteralEnd(b []byte, i int) (int, bool) {
	rest := b[i:]
	switch {
	case len(rest) >= 4 && string(rest[:4]) == "null":
		return i + 4, true
	case len(rest) >= 4 && string(rest[:4]) == "true":
		return i + 4, true
	case len(rest) >= 5 && string(rest[:5]) == "false":
		return i + 5, true
	}
	return i, false
}

func ValueEnd(b []byte, i int, depth int) (int, bool) {
	if i >= len(b) {
		return i, false
	}
	switch c := b[i]; c {
	case '"':
		end, _, ok := StringEnd(b, i+1)
		return end + 1, ok
	case '{':
		return objectEnd(b, i, depth+1)
	case '[':
		return arrayEnd(b, i, depth+1)
	case 'n', 't', 'f':
		return LiteralEnd(b, i)
	default:
		if c == '-' || isDigit(c) {
			return NumberEnd(b, i)
		}
		return i, false
	}
}

func objectEnd(b []byte, i int, depth int) (int, bool) {
	if depth > MaxDepth {
		return i, false
	}
	i = SkipSpace(b, i+1)
	if i < len(b) && b[i] == '}' {
		return i + 1, true
	}
	for {
		if i >= len(b) || b[i] != '"' {
			return i, false
		}
		end, _, ok := StringEnd(b, i+1)
		if !ok {
			return end, false
		}
		i = SkipSpace(b, end+1)
		if i >= len(b) || b[i] != ':' {
			return i, false
		}
		i = SkipSpace(b, i+1)
		if i, ok = ValueEnd(b, i, depth); !ok {
			return i, false
		}
		i = SkipSpace(b, i)
		if i >= len(b) {
			return i, false
		}
		switch b[i] {
		case ',':
			i = SkipSpace(b, i+1)
		case '}':
			return i + 1, true
		default:
			return i, false
		}
	}
}

func arrayEnd(b []byte, i int, depth int) (int, bool) {
	if depth > MaxDepth {
		return i, false
	}
	i = SkipSpace(b, i+1)
	if i < len(b) && b[i] == ']' {
		return i + 1, true
	}
	for {
		var ok bool
		if i, ok = ValueEnd(b, i, depth); !ok {
			return i, false
		}
		i = SkipSpace(b, i)
		if i >= len(b) {
			return i, false
		}
		switch b[i] {
		case ',':
			i = SkipSpace(b, i+1)
		case ']':
			return i + 1, true
		default:
			return i, false
		}
	}
}
