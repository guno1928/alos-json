package scan

import (
	"encoding/binary"
	"math/bits"
	"sync"
	"unsafe"
)

const (
	swarZeros          = 0x3030303030303030
	swarDigitBias      = 0x7676767676767676
	levelWordBits      = 64
	inlineStringWords  = 4
	containerWordShift = 6
)

type deepStack [MaxDepth/levelWordBits + 1]uint64

var deepStackPool = sync.Pool{New: func() any { return new(deepStack) }}

func Valid(b []byte) bool {
	return validFast(b)
}

func validScalar(b []byte) bool {
	end, ok := ValueEnd(b, SkipSpace(b, 0), 0)
	return ok && SkipSpace(b, end) == len(b)
}

func validFast(b []byte) bool {
	var deep *deepStack
	ok := validFastRun(b, &deep)
	if deep != nil {
		deepStackPool.Put(deep)
	}
	return ok
}

func saveLevelWord(deep **deepStack, word int, objects uint64) {
	if *deep == nil {
		*deep = deepStackPool.Get().(*deepStack)
	}
	(*deep)[word] = objects
}

func byteAt(p unsafe.Pointer, i int) byte {
	return *(*byte)(unsafe.Add(p, i))
}

func wordAt(p unsafe.Pointer, i int) uint64 {
	return binary.LittleEndian.Uint64((*[wordSize]byte)(unsafe.Add(p, i))[:])
}

func nonDigitMask(w uint64) uint64 {
	t := w ^ swarZeros
	return (t + swarDigitBias | t) & swarHighs
}

func digitRunEnd(p unsafe.Pointer, n, i int) int {
	for ; i+wordSize <= n; i += wordSize {
		if m := nonDigitMask(wordAt(p, i)); m != 0 {
			return i + bits.TrailingZeros64(m)>>3
		}
	}
	for i < n && byteAt(p, i)-'0' < 10 {
		i++
	}
	return i
}

func nextToken(p unsafe.Pointer, n, i int) (int, byte) {
	if i < n {
		if c := byteAt(p, i); c > ' ' {
			return i, c
		}
		for ; i < n; i++ {
			if c := byteAt(p, i); !isSpace[c] {
				return i, c
			}
		}
	}
	return i, 0
}

func validFastRun(b []byte, deep **deepStack) bool {
	n := len(b)
	p := unsafe.Pointer(unsafe.SliceData(b))
	i := 0
	level := 0
	var objects uint64
	var c byte
	var stop int
value:
	i, c = nextToken(p, n, i)
	switch c {
	case '"':
		i++
		for stop = min(i+inlineStringWords*wordSize, n-wordSize+1); i < stop; i += wordSize {
			if m := SpecialMask(wordAt(p, i)); m != 0 {
				i += bits.TrailingZeros64(m) >> 3
				if byteAt(p, i) == '"' {
					i++
					goto afterValue
				}
				break
			}
		}
		if end, _, ok := StringEnd(b, i); ok {
			i = end + 1
			goto afterValue
		}
		return false
	case '{', '[':
		if level >= MaxDepth {
			return false
		}
		if level&(levelWordBits-1) == 0 && level > 0 {
			saveLevelWord(deep, level>>containerWordShift-1, objects)
		}
		if c == '{' {
			objects |= 1 << (level & (levelWordBits - 1))
		} else {
			objects &^= 1 << (level & (levelWordBits - 1))
		}
		level++
		if c == '{' {
			if i, c = nextToken(p, n, i+1); c == '}' {
				i++
				goto closed
			}
			goto key
		}
		if i, c = nextToken(p, n, i+1); c == ']' {
			i++
			goto closed
		}
		goto value
	case 't':
		if !hasLiteral(b, i, "true") {
			return false
		}
		i += len("true")
	case 'f':
		if !hasLiteral(b, i, "false") {
			return false
		}
		i += len("false")
	case 'n':
		if !hasLiteral(b, i, "null") {
			return false
		}
		i += len("null")
	default:
		goto number
	}
afterValue:
	i, c = nextToken(p, n, i)
	if level == 0 {
		return i == n
	}
	if objects>>((level-1)&(levelWordBits-1))&1 != 0 {
		if c == ',' {
			i, c = nextToken(p, n, i+1)
			goto key
		}
		if c == '}' {
			i++
			goto closed
		}
		return false
	}
	if c == ',' {
		i++
		goto value
	}
	if c == ']' {
		i++
		goto closed
	}
	return false
key:
	if c != '"' {
		return false
	}
	i++
	for stop = min(i+inlineStringWords*wordSize, n-wordSize+1); i < stop; i += wordSize {
		if m := SpecialMask(wordAt(p, i)); m != 0 {
			i += bits.TrailingZeros64(m) >> 3
			if byteAt(p, i) == '"' {
				i++
				goto afterKey
			}
			break
		}
	}
	if end, _, ok := StringEnd(b, i); ok {
		i = end + 1
		goto afterKey
	}
	return false
afterKey:
	if i, c = nextToken(p, n, i); c != ':' {
		return false
	}
	i++
	goto value
closed:
	level--
	if level&(levelWordBits-1) == 0 && level > 0 {
		objects = (*deep)[level>>containerWordShift-1]
	}
	goto afterValue
number:
	if c == '-' {
		if i++; i >= n {
			return false
		}
		c = byteAt(p, i)
	}
	switch {
	case c == '0':
		i++
	case c-'1' >= 9:
		return false
	case i+wordSize > n:
		i = digitRunEnd(p, n, i+1)
	default:
		m := nonDigitMask(wordAt(p, i))
		if m == 0 {
			i = digitRunEnd(p, n, i+wordSize)
			break
		}
		start := i
		if i += bits.TrailingZeros64(m) >> 3; byteAt(p, i) != '.' {
			goto exponent
		}
		fraction := i + 1
		if rest := m & (m - 1); rest != 0 {
			i = start + bits.TrailingZeros64(rest)>>3
		} else {
			i = digitRunEnd(p, n, start+wordSize)
		}
		if i == fraction {
			return false
		}
		goto exponent
	}
	if i < n && byteAt(p, i) == '.' {
		if i++; i >= n || byteAt(p, i)-'0' >= 10 {
			return false
		}
		i = digitRunEnd(p, n, i+1)
	}
exponent:
	if i < n && byteAt(p, i)|0x20 == 'e' {
		if i++; i < n && (byteAt(p, i) == '+' || byteAt(p, i) == '-') {
			i++
		}
		if i >= n || byteAt(p, i)-'0' >= 10 {
			return false
		}
		i = digitRunEnd(p, n, i+1)
	}
	goto afterValue
}

func hasLiteral(b []byte, p int, lit string) bool {
	return p+len(lit) <= len(b) && string(b[p:p+len(lit)]) == lit
}
