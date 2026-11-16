package jsontext

import (
	"bytes"

	"github.com/guno1928/alos-json/internal/scan"
)

var wsTable [256]bool

var hexValTable [256]uint16

const hexDigits = "0123456789abcdef"

func init() {
	wsTable[' '] = true
	wsTable['\t'] = true
	wsTable['\n'] = true
	wsTable['\r'] = true

	for i := range hexValTable {
		hexValTable[i] = 0
	}
	for c := byte('0'); c <= '9'; c++ {
		hexValTable[c] = uint16(c - '0')
	}
	for c := byte('a'); c <= 'f'; c++ {
		hexValTable[c] = uint16(c - 'a' + 10)
	}
	for c := byte('A'); c <= 'F'; c++ {
		hexValTable[c] = uint16(c - 'A' + 10)
	}
}

func consumeWhitespace(b []byte) int {
	return scan.SkipSpace(b, 0)
}

func consumeNull(b []byte) int {
	if len(b) >= 4 && b[0] == 'n' && b[1] == 'u' && b[2] == 'l' && b[3] == 'l' {
		return 4
	}
	return 0
}

func consumeFalse(b []byte) int {
	if len(b) >= 5 && b[0] == 'f' && b[1] == 'a' && b[2] == 'l' && b[3] == 's' && b[4] == 'e' {
		return 5
	}
	return 0
}

func consumeTrue(b []byte) int {
	if len(b) >= 4 && b[0] == 't' && b[1] == 'r' && b[2] == 'u' && b[3] == 'e' {
		return 4
	}
	return 0
}

// FindSafeStringBytes returns the number of leading bytes in b that don't need JSON escaping.
func FindSafeStringBytes(b []byte) int {
	return scan.FindSafeString(b)
}

// FindSafeStringBytesHTML returns the number of leading bytes in b that don't need JSON+HTML escaping.
func FindSafeStringBytesHTML(b []byte) int {
	return scan.FindSafeStringHTML(b)
}

func consumeString(b []byte) int {
	if len(b) == 0 || b[0] != '"' {
		return 0
	}
	end, _, ok := scan.StringEnd(b, 1)
	if !ok {
		return 0
	}
	return end + 1
}

func consumeNumber(b []byte) int {
	end, ok := scan.NumberEnd(b, 0)
	if !ok {
		return 0
	}
	return end
}

func consumeValueBytes(b []byte) int {
	return consumeValueDepth(b, 0)
}

func consumeValueDepth(b []byte, depth int) int {
	end, ok := scan.ValueEnd(b, 0, depth)
	if !ok {
		return 0
	}
	return end
}

func appendUnquote(dst []byte, src []byte) []byte {
	if len(src) < 2 || src[0] != '"' || src[len(src)-1] != '"' {
		return dst
	}
	src = src[1 : len(src)-1]
	i := 0
	for i < len(src) {
		c := src[i]
		if c == '\\' {
			i++
			if i >= len(src) {
				break
			}
			switch src[i] {
			case '"':
				dst = append(dst, '"')
			case '\\':
				dst = append(dst, '\\')
			case '/':
				dst = append(dst, '/')
			case 'b':
				dst = append(dst, '\b')
			case 'f':
				dst = append(dst, '\f')
			case 'n':
				dst = append(dst, '\n')
			case 'r':
				dst = append(dst, '\r')
			case 't':
				dst = append(dst, '\t')
			case 'u':
				if i+4 < len(src) {
					r := hexVal(src[i+1])<<12 | hexVal(src[i+2])<<8 | hexVal(src[i+3])<<4 | hexVal(src[i+4])
					i += 4
					if r >= 0xD800 && r <= 0xDBFF && i+2 < len(src) && src[i+1] == '\\' && src[i+2] == 'u' {
						if i+6 < len(src) {
							r2 := hexVal(src[i+3])<<12 | hexVal(src[i+4])<<8 | hexVal(src[i+5])<<4 | hexVal(src[i+6])
							if r2 >= 0xDC00 && r2 <= 0xDFFF {
								combined := 0x10000 + (rune(r)-0xD800)*0x400 + (rune(r2) - 0xDC00)
								dst = scan.AppendRune(dst, combined)
								i += 6
							} else {
								dst = scan.AppendRune(dst, rune(r))
							}
						} else {
							dst = scan.AppendRune(dst, rune(r))
						}
					} else {
						dst = scan.AppendRune(dst, rune(r))
					}
				}
			}
			i++
			continue
		}
		run := bytes.IndexByte(src[i:], '\\')
		if run < 0 {
			return append(dst, src[i:]...)
		}
		dst = append(dst, src[i:i+run]...)
		i += run
	}
	return dst
}

func hexVal(c byte) uint16 {
	return hexValTable[c]
}

func appendQuotedString(dst []byte, s string) []byte {
	dst = append(dst, '"')
	b := unsafeBytesFromString(s)
	n := FindSafeStringBytes(b)
	if n == len(s) {
		dst = append(dst, s...)
		dst = append(dst, '"')
		return dst
	}
	start := 0
	i := n
	for i < len(s) {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' {
			i++
			continue
		}
		dst = append(dst, s[start:i]...)
		switch c {
		case '"':
			dst = append(dst, '\\', '"')
		case '\\':
			dst = append(dst, '\\', '\\')
		case '\b':
			dst = append(dst, '\\', 'b')
		case '\f':
			dst = append(dst, '\\', 'f')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			dst = append(dst, '\\', 'u', '0', '0', hexChar(c>>4), hexChar(c&0xF))
		}
		i++
		start = i
		if i < len(s) {
			rem := unsafeBytesFromString(s[i:])
			skip := FindSafeStringBytes(rem)
			i += skip
		}
	}
	dst = append(dst, s[start:]...)
	dst = append(dst, '"')
	return dst
}

func hexChar(c byte) byte {
	return hexDigits[c]
}

// AppendQuote appends the JSON-quoted string to dst.
func AppendQuote(dst []byte, s string) []byte {
	return appendQuotedString(dst, s)
}

// AppendUnquote appends the unquoted content of a JSON string to dst.
func AppendUnquote(dst []byte, src []byte) []byte {
	return appendUnquote(dst, src)
}
