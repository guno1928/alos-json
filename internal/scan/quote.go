package scan

const (
	MaxQuoteEscapeLen   = 6
	quoteEscapeTableLen = '\\' + 1
	controlCharLimit    = 0x20
	hexDigitsLower      = "0123456789abcdef"
	escapeLenShift      = 56
	bitsPerByte         = 8
	nibbleBits          = 4
	nibbleMask          = 0xF
)

var quoteEscapes = buildQuoteEscapes()

func buildQuoteEscapes() (table [quoteEscapeTableLen]uint64) {
	for c := range controlCharLimit {
		table[c] = packEscape(string([]byte{'\\', 'u', '0', '0', hexDigitsLower[c>>nibbleBits], hexDigitsLower[c&nibbleMask]}))
	}
	for c, seq := range map[byte]string{'"': `\"`, '\\': `\\`, '\b': `\b`, '\f': `\f`, '\n': `\n`, '\r': `\r`, '\t': `\t`} {
		table[c] = packEscape(seq)
	}
	return table
}

func packEscape(seq string) uint64 {
	packed := uint64(len(seq)) << escapeLenShift
	for i := range len(seq) {
		packed |= uint64(seq[i]) << (bitsPerByte * i)
	}
	return packed
}

func needsQuoteEscape(c byte) bool {
	return c < controlCharLimit || c == '"' || c == '\\'
}

func quoteEscapeLen(packed uint64) int {
	return int(packed >> escapeLenShift)
}

func quoteEscapeGeneric(dst []byte, src string) (written, consumed int) {
	out := dst[len(dst):cap(dst)]
	for consumed < len(src) {
		c := src[consumed]
		if !needsQuoteEscape(c) {
			if written == len(out) {
				break
			}
			out[written] = c
			written++
			consumed++
			continue
		}
		packed := quoteEscapes[c]
		n := quoteEscapeLen(packed)
		if written+n > len(out) {
			break
		}
		for i := range n {
			out[written+i] = byte(packed >> (bitsPerByte * i))
		}
		written += n
		consumed++
	}
	return written, consumed
}
