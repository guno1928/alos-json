package jsontext

const maxInternLen = 256

type stringCache [256]string

func (c *stringCache) owned(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b) > maxInternLen {
		return string(b)
	}
	idx := stringHash(b) & 0xFF
	if c[idx] == string(b) {
		return c[idx]
	}
	s := string(b)
	c[idx] = s
	return s
}

func stringHash(b []byte) uint32 {
	const (
		p1 = 0x9E3779B1
		p2 = 0x85EBCA77
	)
	n := len(b)
	var h uint32
	switch {
	case n >= 8:
		h = uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
		h *= p1
		h2 := uint32(b[n-4]) | uint32(b[n-3])<<8 | uint32(b[n-2])<<16 | uint32(b[n-1])<<24
		h ^= h2 * p2
	case n >= 4:
		h = uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24
		h *= p1
	default:
		h = uint32(b[0]) | uint32(b[n-1])<<8
		h *= p1
	}
	h ^= h >> 15
	h *= p2
	h ^= h >> 13
	return h
}
