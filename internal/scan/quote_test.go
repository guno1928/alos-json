package scan

import (
	"bytes"
	"strings"
	"testing"
)

const (
	quoteCanary       = 0xA5
	quoteCanaryLen    = 64
	quoteMaxTestLen   = 130
	quoteAlignSpan    = 64
	quoteRoomSweepLen = 40
	quoteDoubleRoom   = 70
)

var quoteSpecialSamples = []byte{0x00, 0x01, '\b', '\t', '\n', '\f', '\r', 0x1F, '"', '\\'}

var quoteNeighbourSamples = []byte{0x20, '!', '#', '/', '<', '>', '&', '[', ']', 0x7F, 0x80, 0xC3, 0xE2, 0xFF}

var quoteRefScratch, quoteCanaryScratch []byte

func referenceQuoteEscape(src string) []byte {
	if need := len(src) * MaxQuoteEscapeLen; cap(quoteRefScratch) < need {
		quoteRefScratch = make([]byte, 0, need)
	}
	out := quoteRefScratch[:0]
	written, consumed := quoteEscapeGeneric(out, src)
	if consumed != len(src) {
		panic("reference quoteEscapeGeneric did not finish with worst-case room")
	}
	return out[:written]
}

func checkQuoteEscape(t *testing.T, src string, room, prefixLen int) {
	t.Helper()
	buf := canaryBuffer(prefixLen + room + quoteCanaryLen)
	dst := buf[: prefixLen : prefixLen+room]
	written, consumed := QuoteEscape(dst, src)
	if consumed < 0 || consumed > len(src) || written < 0 || written > room {
		t.Fatalf("QuoteEscape(%q, room=%d) = (%d, %d) out of range", src, room, written, consumed)
	}
	if want := referenceQuoteEscape(src[:consumed]); !bytes.Equal(buf[prefixLen:prefixLen+written], want) {
		t.Fatalf("QuoteEscape(%q, room=%d) wrote %q, want %q", src, room, buf[prefixLen:prefixLen+written], want)
	}
	untouchedFrom := prefixLen + room
	if consumed == len(src) {
		untouchedFrom = prefixLen + written
	}
	for i, c := range buf[untouchedFrom:] {
		if c != quoteCanary {
			t.Fatalf("QuoteEscape(%q, room=%d) wrote outside its output at byte %d", src, room, untouchedFrom+i)
		}
	}
	for i, c := range buf[:prefixLen] {
		if c != quoteCanary {
			t.Fatalf("QuoteEscape(%q, room=%d) wrote into the prefix at byte %d", src, room, i)
		}
	}
	if len(src) > 0 && room >= len(src)+MaxQuoteEscapeLen && consumed == 0 {
		t.Fatalf("QuoteEscape(%q, room=%d) made no progress", src, room)
	}
	if room >= len(src)*MaxQuoteEscapeLen+MaxQuoteEscapeLen && consumed != len(src) {
		t.Fatalf("QuoteEscape(%q, room=%d) stopped at %d with worst-case room", src, room, consumed)
	}
}

func canaryBuffer(n int) []byte {
	if cap(quoteCanaryScratch) < n {
		quoteCanaryScratch = make([]byte, n)
	}
	buf := quoteCanaryScratch[:n]
	for i := range buf {
		buf[i] = quoteCanary
	}
	return buf
}

func checkQuoteEscapeFullRoom(t *testing.T, src string) {
	t.Helper()
	checkQuoteEscape(t, src, len(src)*MaxQuoteEscapeLen+MaxQuoteEscapeLen, 0)
	checkQuoteEscape(t, src, len(src)+MaxQuoteEscapeLen, 1)
}

func quoteSampleBytes() []byte {
	return append(append([]byte{'a'}, quoteSpecialSamples...), quoteNeighbourSamples...)
}

func TestQuoteEscapeEveryByteAtEveryPosition(t *testing.T) {
	checkQuoteEscapeFullRoom(t, "")
	for n := 1; n <= quoteMaxTestLen; n++ {
		base := bytes.Repeat([]byte{'a'}, n)
		checkQuoteEscapeFullRoom(t, string(base))
		for pos := range n {
			for c := range 256 {
				b := append([]byte(nil), base...)
				b[pos] = byte(c)
				checkQuoteEscapeFullRoom(t, string(b))
			}
		}
	}
}

func TestQuoteEscapeDenseAndPairs(t *testing.T) {
	samples := quoteSampleBytes()
	for n := 0; n <= quoteMaxTestLen; n++ {
		for _, c := range samples {
			checkQuoteEscapeFullRoom(t, strings.Repeat(string(c), n))
		}
		for _, a := range quoteSpecialSamples {
			for _, b := range samples {
				var sb strings.Builder
				for i := range n {
					if i%2 == 0 {
						sb.WriteByte(a)
					} else {
						sb.WriteByte(b)
					}
				}
				checkQuoteEscapeFullRoom(t, sb.String())
			}
		}
	}
}

var quoteSweepSpecials = []byte{0x00, 0x0A, 0x22, 0x5C}

var quoteSweepFillers = []byte{'a', 0x80, '"'}

func TestQuoteEscapeRoomSweep(t *testing.T) {
	for n := 0; n <= quoteRoomSweepLen; n++ {
		for _, special := range quoteSweepSpecials {
			for _, filler := range quoteSweepFillers {
				for pos := 0; pos <= n; pos += 3 {
					b := bytes.Repeat([]byte{filler}, n)
					if pos < n {
						b[pos] = special
					}
					for room := 0; room <= n*MaxQuoteEscapeLen+MaxQuoteEscapeLen; room++ {
						checkQuoteEscape(t, string(b), room, pos%3)
					}
				}
			}
		}
	}
}

var quoteLongSweepLens = []int{63, 64, 65, 70, 95, 96, 97, 127, 128, 129, 130}

func TestQuoteEscapeRoomSweepLong(t *testing.T) {
	for _, n := range quoteLongSweepLens {
		for _, special := range quoteSweepSpecials {
			for _, pos := range []int{0, 1, 15, 16, 31, 32, 33, 63, 64, n - 1, n} {
				if pos > n {
					continue
				}
				b := bytes.Repeat([]byte{'a'}, n)
				if pos < n {
					b[pos] = special
				}
				for room := 0; room <= n+2*quoteDoubleRoom; room++ {
					checkQuoteEscape(t, string(b), room, pos%3)
				}
			}
		}
	}
}

func TestQuoteEscapeUnalignedSources(t *testing.T) {
	var sb strings.Builder
	for i := range quoteMaxTestLen + quoteAlignSpan {
		samples := quoteSampleBytes()
		sb.WriteByte(samples[(i*7)%len(samples)])
	}
	big := sb.String()
	clean := strings.Repeat("abcdefgh", (quoteMaxTestLen+quoteAlignSpan)/8+1)
	for off := range quoteAlignSpan {
		for n := 0; n <= quoteMaxTestLen; n++ {
			checkQuoteEscapeFullRoom(t, big[off:off+n])
			checkQuoteEscapeFullRoom(t, clean[off:off+n])
		}
	}
}

func TestQuoteEscapeUTF8AndSeparators(t *testing.T) {
	for _, s := range []string{
		"café naïve résumé",
		"日本語のテキストです",
		"emoji 😀🎉 party",
		"\u2028\u2029",
		"line\u2028sep\u2029para",
		"\xff\xfe\xfd",
		"\xc3\x28",
		"\xe2\x80",
		"<script>alert('x')</script> & more",
		`He said "hello" and left.`,
		`C:\Users\path\to\file`,
		"控制字符\u0001\u001f",
	} {
		for reps := 1; reps <= 9; reps++ {
			checkQuoteEscapeFullRoom(t, strings.Repeat(s, reps))
		}
	}
}

func FuzzQuoteEscape(f *testing.F) {
	f.Add([]byte(`He said "hello"`), uint16(64), uint8(0))
	f.Add([]byte("tab\tsep\x01\x1f"), uint16(3), uint8(5))
	f.Add(bytes.Repeat([]byte{'"'}, 70), uint16(200), uint8(1))
	f.Add([]byte("日本語\u2028"), uint16(0), uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, room uint16, prefix uint8) {
		s := string(data)
		checkQuoteEscape(t, s, int(room), int(prefix%8))
		checkQuoteEscapeFullRoom(t, s)
	})
}
