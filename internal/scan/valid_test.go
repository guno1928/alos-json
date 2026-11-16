package scan

import (
	"encoding/binary"
	"math/bits"
	"strconv"
	"strings"
	"testing"
	"unsafe"
)

func firstNonDigitReference(b []byte) int {
	for i, c := range b {
		if c < '0' || c > '9' {
			return i
		}
	}
	return len(b)
}

func TestNonDigitMaskFirstIsExact(t *testing.T) {
	var buf [wordSize]byte
	check := func() {
		t.Helper()
		got := wordSize
		if m := nonDigitMask(binary.LittleEndian.Uint64(buf[:])); m != 0 {
			got = bits.TrailingZeros64(m) >> 3
		}
		if want := firstNonDigitReference(buf[:]); got != want {
			t.Fatalf("nonDigitMask(%q) first = %d, want %d", buf, got, want)
		}
	}
	for pos := range wordSize {
		for c := range 256 {
			for k := range buf {
				buf[k] = '0' + byte(k%10)
			}
			buf[pos] = byte(c)
			check()
			for d := range 256 {
				if pos+1 < wordSize {
					buf[pos+1] = byte(d)
					check()
				}
			}
		}
	}
	x := uint64(0x452821E638D01377)
	for range 200_000 {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		binary.LittleEndian.PutUint64(buf[:], x)
		check()
	}
}

func TestDigitRunEndMatchesReference(t *testing.T) {
	x := uint64(0xBE5466CF34E90C6C)
	for range 50_000 {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		n := int(x % 40)
		b := make([]byte, n)
		for i := range b {
			x ^= x >> 12
			x ^= x << 25
			x ^= x >> 27
			if x%5 == 0 {
				b[i] = byte(x >> 40)
			} else {
				b[i] = '0' + byte(x>>40)%10
			}
		}
		for start := 0; start <= n; start++ {
			want := start + firstNonDigitReference(b[start:])
			if n == 0 {
				if got := digitRunEnd(nil, 0, 0); got != 0 {
					t.Fatalf("empty: got %d", got)
				}
				continue
			}
			if got := digitRunEnd(unsafe.Pointer(unsafe.SliceData(b)), n, start); got != want {
				t.Fatalf("digitRunEnd(%q, %d) = %d, want %d", b, start, got, want)
			}
		}
	}
}

func checkValid(t *testing.T, b []byte) {
	t.Helper()
	if got, want := validFast(b), validScalar(b); got != want {
		t.Fatalf("validFast(%q) = %v, validScalar = %v", b, got, want)
	}
}

const validAlphabet = "{}[]:,\" \\1-.e0tnux\x01\xc3a"

func TestValidFastExhaustiveShort(t *testing.T) {
	var buf [5]byte
	pads := []int{0, 61, 63, 64, 125}
	var walk func(n int)
	walk = func(n int) {
		body := string(buf[:n])
		for _, pad := range pads {
			checkValid(t, []byte(strings.Repeat(" ", pad)+body))
			checkValid(t, []byte(strings.Repeat(" ", pad)+`["`+strings.Repeat("s", pad)+`",`+body+`]`))
		}
		if n == len(buf) {
			return
		}
		for j := 0; j < len(validAlphabet); j++ {
			buf[n] = validAlphabet[j]
			walk(n + 1)
		}
	}
	walk(0)
}

type docGen struct{ x uint64 }

func (g *docGen) next() uint64 {
	g.x ^= g.x >> 12
	g.x ^= g.x << 25
	g.x ^= g.x >> 27
	return g.x * 2685821657736338717
}

var genStrings = []string{`""`, `"a"`, `"key"`, `"esc\n\t\"\\"`, `"\u00e9\uD83D\uDE00"`, `"` + strings.Repeat("long", 40) + `"`, `"\/"`, "\"é日本\""}
var genScalars = []string{"0", "-1", "12.5", "1e10", "-0.25E-3", "true", "false", "null", "123456789012345678901234567890"}
var genSpace = []string{"", "", "", " ", "\n", "\t ", "\r\n  "}

func (g *docGen) value(sb *strings.Builder, depth int) {
	sb.WriteString(genSpace[g.next()%uint64(len(genSpace))])
	switch r := g.next() % 10; {
	case depth > 6 || r < 4:
		if r%2 == 0 {
			sb.WriteString(genStrings[g.next()%uint64(len(genStrings))])
		} else {
			sb.WriteString(genScalars[g.next()%uint64(len(genScalars))])
		}
	case r < 7:
		sb.WriteByte('[')
		for i := range int(g.next() % 5) {
			if i > 0 {
				sb.WriteByte(',')
			}
			g.value(sb, depth+1)
		}
		sb.WriteByte(']')
	default:
		sb.WriteByte('{')
		for i := range int(g.next() % 5) {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(genSpace[g.next()%uint64(len(genSpace))])
			sb.WriteString(genStrings[g.next()%uint64(len(genStrings))])
			sb.WriteString(genSpace[g.next()%uint64(len(genSpace))])
			sb.WriteByte(':')
			g.value(sb, depth+1)
		}
		sb.WriteByte('}')
	}
	sb.WriteString(genSpace[g.next()%uint64(len(genSpace))])
}

var mutationBytes = []byte("{}[]:,\"\\ 0a\x00\x1f\xff-.eu")

func TestValidFastRandomDocsAndMutations(t *testing.T) {
	g := &docGen{x: 0x13198A2E03707344}
	valid := 0
	for range 4_000 {
		var sb strings.Builder
		g.value(&sb, 0)
		doc := []byte(sb.String())
		checkValid(t, doc)
		if validScalar(doc) {
			valid++
		}
		for range 12 {
			m := append([]byte(nil), doc...)
			pos := int(g.next() % uint64(len(m)))
			switch g.next() % 3 {
			case 0:
				m[pos] = mutationBytes[g.next()%uint64(len(mutationBytes))]
			case 1:
				m = append(m[:pos], m[pos+1:]...)
			default:
				m = append(m[:pos], append([]byte{mutationBytes[g.next()%uint64(len(mutationBytes))]}, m[pos:]...)...)
			}
			checkValid(t, m)
		}
	}
	if valid < 3_900 {
		t.Fatalf("generator produced only %d valid documents", valid)
	}
}

func TestValidFastDepth(t *testing.T) {
	for _, d := range []int{1, 2, 63, 64, 65, 255, 256, 257, 300, MaxDepth - 1, MaxDepth, MaxDepth + 1} {
		arr := strings.Repeat("[", d) + strings.Repeat("]", d)
		obj := strings.Repeat(`{"k":`, d) + "1" + strings.Repeat("}", d)
		mixed := strings.Repeat(`[{"a":`, d/2+1) + "0" + strings.Repeat("}]", d/2+1)
		for _, doc := range []string{arr, obj, mixed, arr + " ", " " + obj, arr[:len(arr)-1], obj + "}"} {
			checkValid(t, []byte(doc))
		}
	}
}

func TestValidFastTailAlignment(t *testing.T) {
	for n := range 300 {
		s := `{"k":[` + strings.Repeat("1,", n) + `"` + strconv.Itoa(n) + `"]}`
		for _, doc := range []string{s, s + " ", " " + s, s[:len(s)-1], s + "x", s[:len(s)/2]} {
			checkValid(t, []byte(doc))
		}
	}
}

func FuzzValidFast(f *testing.F) {
	g := &docGen{x: 7}
	for range 40 {
		var sb strings.Builder
		g.value(&sb, 0)
		f.Add(sb.String())
	}
	f.Fuzz(func(t *testing.T, s string) {
		checkValid(t, []byte(s))
	})
}

func TestValidFastDepthWindowsOfDifferentKinds(t *testing.T) {
	for _, outer := range []int{1, 63, 64, 65, 127, 128, 129, 200} {
		for _, inner := range []int{1, 63, 64, 65, 130} {
			arraysThenObjects := strings.Repeat("[", outer) + strings.Repeat(`{"k":`, inner) + "1" + strings.Repeat("}", inner) + strings.Repeat("]", outer)
			objectsThenArrays := strings.Repeat(`{"k":`, outer) + strings.Repeat("[", inner) + "1" + strings.Repeat("]", inner) + strings.Repeat("}", outer)
			for _, doc := range []string{
				arraysThenObjects,
				objectsThenArrays,
				arraysThenObjects[:len(arraysThenObjects)-1] + "}",
				objectsThenArrays[:len(objectsThenArrays)-1] + "]",
				strings.Replace(arraysThenObjects, "}]", "}],2]", 1),
				strings.Replace(objectsThenArrays, "]}", `],"x":3}`, 1),
				strings.Replace(objectsThenArrays, "]}", "],3}", 1),
			} {
				checkValid(t, []byte(doc))
			}
		}
	}
}

func TestNonDigitMaskSecondIsExactAfterDot(t *testing.T) {
	var buf [wordSize]byte
	for dot := 1; dot < wordSize; dot++ {
		for end := dot + 1; end <= wordSize; end++ {
			for c := range 256 {
				for k := range buf {
					buf[k] = '1' + byte(k%9)
				}
				buf[dot] = '.'
				for k := end + 1; k < wordSize; k++ {
					buf[k] = byte(c ^ k*37)
				}
				if end < wordSize {
					buf[end] = byte(c)
				}
				m := nonDigitMask(binary.LittleEndian.Uint64(buf[:]))
				got := wordSize
				if rest := m & (m - 1); rest != 0 {
					got = bits.TrailingZeros64(rest) >> 3
				}
				want := dot + 1 + firstNonDigitReference(buf[dot+1:])
				if bits.TrailingZeros64(m)>>3 != dot || got != want {
					t.Fatalf("nonDigitMask(%q): first %d second %d, want %d %d", buf, bits.TrailingZeros64(m)>>3, got, dot, want)
				}
			}
		}
	}
}
