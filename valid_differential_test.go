package json_test

import (
	stdjson "encoding/json"
	"strings"
	"testing"

	alosjson "github.com/guno1928/alos-json"
	"github.com/guno1928/alos-json/jsontext"
)

const stdMaxDepth = 10000

const kernelPadding = 4096

func blankStringContents(b []byte) []byte {
	out := make([]byte, len(b))
	inString := false
	for i := 0; i < len(b); i++ {
		c := b[i]
		switch {
		case !inString:
			out[i] = c
			inString = c == '"'
		case c == '"':
			out[i] = c
			inString = false
		case c == '\\' && i+1 < len(b):
			out[i], out[i+1] = 'a', 'a'
			i++
		default:
			out[i] = 'a'
		}
	}
	return out
}

func checkLaxValid(t *testing.T, b []byte) {
	t.Helper()
	want := stdjson.Valid(blankStringContents(b))
	if got := alosjson.Valid(b); got != want {
		t.Fatalf("Valid(%q) = %v, encoding/json with string contents blanked = %v", b, got, want)
	}
}

func checkValidAgainstStd(t *testing.T, b []byte) {
	t.Helper()
	if got, want := jsontext.Value(b).IsValid(), stdjson.Valid(b); got != want {
		t.Fatalf("Value(%q).IsValid() = %v, encoding/json = %v", b, got, want)
	}
	checkLaxValid(t, b)
	checkLaxValid(t, append([]byte(strings.Repeat(" ", kernelPadding)), b...))
}

func TestValidSkipsStringContents(t *testing.T) {
	for _, s := range []string{`"\x"`, `"\u12g4"`, `"\u12"`, "\"a\x01\"", `"\'"`, `{"\q":"\uZZZZ"}`, "[\"\t\",\"\x00\"]", `"\\"`, `"\""`} {
		if !alosjson.Valid([]byte(s)) {
			t.Errorf("Valid(%q) = false, want true", s)
		}
		if jsontext.Value(s).IsValid() != stdjson.Valid([]byte(s)) {
			t.Errorf("Value(%q).IsValid() disagrees with encoding/json", s)
		}
	}
	for _, s := range []string{`"\"`, `"abc`, `"\`, `["\"]`, `{"a\":1}`, `"a"x`, `"\x" 1`} {
		if alosjson.Valid([]byte(s)) {
			t.Errorf("Valid(%q) = true, want false", s)
		}
	}
}

var validEdgeCases = []string{
	"", " ", "\t\n\r ", "null", "true", "false", "nul", "tru", "fals", "nulll", "truee", "True", "NULL",
	"0", "-0", "00", "01", "-01", "1.", ".1", "1.e1", "1e", "1e+", "1e-", "1E5", "1e05", "-", "--1", "+1",
	"0.0", "-0.0e-0", "123456789012345678901234567890", "1.5e999999", "0x10", "1_000", "Infinity", "NaN",
	`""`, `"`, `"\"`, `"\\"`, `"\u0041"`, `"\u004"`, `"\u00G1"`, `"\uD800"`, `"\uDFFF\uD800"`, `"\x"`, `"\'"`,
	"\"\x00\"", "\"\x1f\"", "\"\x7f\"", "\"\xff\"", "\"\xc3\"", "\"\xed\xa0\x80\"", "\"é日本😀\"",
	"[]", "{}", "[", "]", "{", "}", "[,]", "[1,]", "[,1]", "[1,,2]", "[1 2]", "[1]]", "[[1]", `{"a"}`, `{"a":}`,
	`{"a":1,}`, `{,"a":1}`, `{"a" 1}`, `{"a":1 "b":2}`, `{1:2}`, `{"a":1}}`, `{"a":[1,{"b":null}]}`,
	" [ 1 , 2 ] ", "[1]\x00", "\x00[1]", "[1] x", "1 2", "{} {}", "\ufeff{}", "[\"a\"\"b\"]", "[true false]",
	"[1e1]", "[-]", "[-1.5E+3]", "[0.]", "[1.2.3]", "[\"\t\"]", "{\"\\u0000\":0}", "[\"\\/\"]",
}

func TestValidEdgeCasesMatchEncodingJSON(t *testing.T) {
	for _, s := range validEdgeCases {
		checkValidAgainstStd(t, []byte(s))
		checkValidAgainstStd(t, []byte("["+s+"]"))
		checkValidAgainstStd(t, []byte(`{"k":`+s+`}`))
		checkValidAgainstStd(t, []byte(`["`+strings.Repeat("p", 70)+`",`+s+`]`))
	}
}

func TestValidDepthMatchesEncodingJSON(t *testing.T) {
	for _, d := range []int{1, 63, 64, 65, 128, 129, 1000, stdMaxDepth - 1, stdMaxDepth, stdMaxDepth + 1, stdMaxDepth + 100} {
		checkValidAgainstStd(t, []byte(strings.Repeat("[", d)+strings.Repeat("]", d)))
		checkValidAgainstStd(t, []byte(strings.Repeat(`{"k":`, d)+"0"+strings.Repeat("}", d)))
		checkValidAgainstStd(t, []byte(strings.Repeat(`[{"k":`, d/2)+"0"+strings.Repeat("}]", d/2)))
		checkValidAgainstStd(t, []byte(strings.Repeat("[", d)))
	}
}

type validDocGen struct{ x uint64 }

func (g *validDocGen) next() uint64 {
	g.x ^= g.x >> 12
	g.x ^= g.x << 25
	g.x ^= g.x >> 27
	return g.x * 2685821657736338717
}

func (g *validDocGen) pick(options []string) string {
	return options[g.next()%uint64(len(options))]
}

var (
	genValidStrings  = []string{`""`, `"a"`, `"key"`, `"esc\n\t\"\\\/\b\f\r"`, `"\u00e9\uD83D\uDE00\u003c"`, `"` + strings.Repeat("long", 40) + `"`, "\"é日本\"", "\"\xff\xfe\"", `"` + strings.Repeat(`\u0026x\"`, 30) + `"`}
	genValidScalars  = []string{"0", "-1", "12.5", "1e10", "-0.25E-3", "true", "false", "null", "123456789012345678901234567890", "3.141592653589793", "-9223372036854775808"}
	genValidSpace    = []string{"", "", "", " ", "\n", "\t ", "\r\n  "}
	mutationAlphabet = []byte("{}[]:,\"\\ 0123456789aeflnrstu-.+E\x00\x1f\x7f\xff\xc3")
)

func (g *validDocGen) value(sb *strings.Builder, depth int) {
	sb.WriteString(g.pick(genValidSpace))
	switch r := g.next() % 10; {
	case depth > 6 || r < 4:
		if r%2 == 0 {
			sb.WriteString(g.pick(genValidStrings))
		} else {
			sb.WriteString(g.pick(genValidScalars))
		}
	case r < 7:
		sb.WriteByte('[')
		for i := range int(g.next() % 6) {
			if i > 0 {
				sb.WriteByte(',')
			}
			g.value(sb, depth+1)
		}
		sb.WriteByte(']')
	default:
		sb.WriteByte('{')
		for i := range int(g.next() % 6) {
			if i > 0 {
				sb.WriteByte(',')
			}
			sb.WriteString(g.pick(genValidSpace) + g.pick(genValidStrings) + g.pick(genValidSpace) + ":")
			g.value(sb, depth+1)
		}
		sb.WriteByte('}')
	}
	sb.WriteString(g.pick(genValidSpace))
}

func (g *validDocGen) mutate(doc []byte) []byte {
	m := append([]byte(nil), doc...)
	for range 1 + g.next()%3 {
		if len(m) == 0 {
			return m
		}
		pos := int(g.next() % uint64(len(m)))
		c := mutationAlphabet[g.next()%uint64(len(mutationAlphabet))]
		switch g.next() % 3 {
		case 0:
			m[pos] = c
		case 1:
			m = append(m[:pos], m[pos+1:]...)
		default:
			m = append(m[:pos], append([]byte{c}, m[pos:]...)...)
		}
	}
	return m
}

func TestValidRandomDocsMatchEncodingJSON(t *testing.T) {
	g := &validDocGen{x: 0x9E3779B97F4A7C15}
	valid := 0
	for range 20_000 {
		var sb strings.Builder
		g.value(&sb, 0)
		doc := []byte(sb.String())
		checkValidAgainstStd(t, doc)
		if stdjson.Valid(doc) {
			valid++
		}
		for range 8 {
			checkValidAgainstStd(t, g.mutate(doc))
		}
	}
	if valid < 19_000 {
		t.Fatalf("generator produced only %d valid documents", valid)
	}
}

func FuzzValidDifferential(f *testing.F) {
	for _, s := range validEdgeCases {
		f.Add(s)
	}
	g := &validDocGen{x: 42}
	for range 50 {
		var sb strings.Builder
		g.value(&sb, 0)
		f.Add(sb.String())
	}
	f.Fuzz(func(t *testing.T, s string) {
		checkValidAgainstStd(t, []byte(s))
	})
}
