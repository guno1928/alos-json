//go:build amd64 && !purego

package scan

import (
	"bytes"
	"math/bits"
	"unsafe"
)

const (
	tokenBatchLen           = 1024
	densitySampleLen        = 4 * BlockLen
	sparseBytesPerSeparator = 32
)

var (
	commaByte = []byte{','}
	colonByte = []byte{':'}
)

type blockState struct {
	pos         int
	inStrMask   uint64
	escCarry    uint64
	hexCarry    uint64
	scalarCarry uint64
}

var blankBlock = func() (blk [BlockLen]byte) {
	for i := range blk {
		blk[i] = ' '
	}
	return blk
}()

var scalarTerminator = [256]bool{' ': true, '\t': true, '\n': true, '\r': true, ',': true, ':': true, '[': true, ']': true, '{': true, '}': true, '"': true}

const batchEnd = ^uint32(0)

type tokenStream struct {
	b   []byte
	src unsafe.Pointer
	st  blockState
	idx [tokenBatchLen + 1]uint32
}

func takeToken(src, ip unsafe.Pointer) (int, byte, unsafe.Pointer) {
	i := int(int32(*(*uint32)(ip)))
	if i < 0 {
		return -1, 0, ip
	}
	return i, byteAt(src, i), unsafe.Add(ip, 4)
}

func (s *tokenStream) refill() (int, byte, unsafe.Pointer) {
	for s.st.pos < len(s.b) {
		cnt := s.fill()
		if cnt < 0 {
			break
		}
		if cnt > 0 {
			s.idx[cnt] = batchEnd
			i := int(s.idx[0])
			return i, byteAt(s.src, i), unsafe.Pointer(&s.idx[1])
		}
	}
	s.idx[0] = batchEnd
	if s.st.pos < len(s.b) || s.st.inStrMask>>63 != 0 {
		return -1, 0, unsafe.Pointer(&s.idx[0])
	}
	return len(s.b), 0, unsafe.Pointer(&s.idx[0])
}

func (s *tokenStream) fill() int {
	full := len(s.b) &^ (BlockLen - 1)
	if s.st.pos < full {
		return validBlocksStrict((*byte)(unsafe.Add(s.src, s.st.pos)), full-s.st.pos, &s.st, &s.idx[0], tokenBatchLen)
	}
	tail := blankBlock
	copy(tail[:], s.b[s.st.pos:])
	return validBlocksStrict(&tail[0], BlockLen, &s.st, &s.idx[0], tokenBatchLen)
}

func (s *tokenStream) scalarEnds(i int) bool {
	return i >= len(s.b) || scalarTerminator[byteAt(s.src, i)]
}

func validAdaptive(b []byte) bool {
	sample := b[:min(len(b), densitySampleLen)]
	if (bytes.Count(sample, commaByte)+bytes.Count(sample, colonByte))*sparseBytesPerSeparator > len(sample) {
		return validFast(b)
	}
	return validIndexed(b)
}

func validIndexed(b []byte) bool {
	s := tokenStream{b: b, src: unsafe.Pointer(unsafe.SliceData(b))}
	s.idx[0] = batchEnd
	var deep *deepStack
	ok := s.run(&deep)
	if deep != nil {
		deepStackPool.Put(deep)
	}
	return ok
}

func (s *tokenStream) run(deep **deepStack) bool {
	n := len(s.b)
	src := s.src
	ip := unsafe.Pointer(&s.idx[0])
	level := 0
	var objects uint64
	var i int
	var c byte
fetchValue:
	if i, c, ip = takeToken(src, ip); i < 0 {
		i, c, ip = s.refill()
	}
value:
	switch c {
	case '"':
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
		open := c
		if i, c, ip = takeToken(src, ip); i < 0 {
			i, c, ip = s.refill()
		}
		if c == open+2 {
			goto closed
		}
		if open == '{' {
			goto key
		}
		goto value
	case 't':
		if !hasLiteral(s.b, i, "true") || !s.scalarEnds(i+len("true")) {
			return false
		}
	case 'f':
		if !hasLiteral(s.b, i, "false") || !s.scalarEnds(i+len("false")) {
			return false
		}
	case 'n':
		if !hasLiteral(s.b, i, "null") || !s.scalarEnds(i+len("null")) {
			return false
		}
	default:
		if i = scanNumber(src, n, i, c); i < 0 || !s.scalarEnds(i) {
			return false
		}
	}
afterValue:
	if i, c, ip = takeToken(src, ip); i < 0 {
		i, c, ip = s.refill()
	}
	if level == 0 {
		return i == n
	}
	if objects>>((level-1)&(levelWordBits-1))&1 != 0 {
		if c == ',' {
			goto member
		}
		if c == '}' {
			goto closed
		}
		return false
	}
	if c == ',' {
		goto fetchValue
	}
	if c == ']' {
		goto closed
	}
	return false
member:
	if i, c, ip = takeToken(src, ip); i < 0 {
		_, c, ip = s.refill()
	}
key:
	if c != '"' {
		return false
	}
	if i, c, ip = takeToken(src, ip); i < 0 {
		_, c, ip = s.refill()
	}
	if c != ':' {
		return false
	}
	goto fetchValue
closed:
	level--
	if level&(levelWordBits-1) == 0 && level > 0 {
		objects = (*deep)[level>>containerWordShift-1]
	}
	goto afterValue
}

func scanNumber(p unsafe.Pointer, n, i int, c byte) int {
	if c == '-' {
		if i++; i >= n {
			return -1
		}
		c = byteAt(p, i)
	}
	switch {
	case c == '0':
		i++
	case c-'1' >= 9:
		return -1
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
			return -1
		}
		goto exponent
	}
	if i < n && byteAt(p, i) == '.' {
		if i++; i >= n || byteAt(p, i)-'0' >= 10 {
			return -1
		}
		i = digitRunEnd(p, n, i+1)
	}
exponent:
	if i < n && byteAt(p, i)|0x20 == 'e' {
		if i++; i < n && (byteAt(p, i) == '+' || byteAt(p, i) == '-') {
			i++
		}
		if i >= n || byteAt(p, i)-'0' >= 10 {
			return -1
		}
		i = digitRunEnd(p, n, i+1)
	}
	return i
}
