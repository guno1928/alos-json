package scan

import (
	"encoding/json"
	"strings"
	"testing"
)

const laxKernelPadding = 4096

func blankStringContentsReference(b []byte) []byte {
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

func laxSpec(b []byte) bool {
	return json.Valid(blankStringContentsReference(b))
}

var laxStringBodies = []string{
	``, `a`, `\x`, `\u12g4`, `\u12`, `\u`, `\'`, `\\`, `\"`, `\/`, "\x00", "\x01\x1f", "\t\n", "\xff\xfe", "é日本😀",
	`\uD800`, `\\\"`, `\\\\`, `ab\`, `\`, `"`, `a"b`, strings.Repeat(`\x`, 40), strings.Repeat("\x01", 70),
	strings.Repeat(`&x\"`, 30), strings.Repeat(`\\`, 33) + `\`, strings.Repeat("p", 63) + `\`, strings.Repeat("p", 64) + `\"`,
}

func laxCorpus() [][]byte {
	var docs [][]byte
	for _, body := range laxStringBodies {
		s := `"` + body + `"`
		for _, doc := range []string{s, "[" + s + "]", `{` + s + `:` + s + `}`, `[1,` + s + `,{"k":` + s + `}]`, s + `x`, `[` + s, `{` + s + `}`, `{` + s + `,1}`} {
			docs = append(docs, []byte(doc))
		}
	}
	return docs
}

func checkLax(t *testing.T, b []byte) {
	t.Helper()
	want := laxSpec(b)
	if got := ValidLax(b); got != want {
		t.Fatalf("ValidLax (len %d) %q = %v, spec = %v", len(b), b, got, want)
	}
	if got := validFastLax(b); got != want {
		t.Fatalf("validFast lax (len %d) %q = %v, spec = %v", len(b), b, got, want)
	}
	if got, want := Valid(b), json.Valid(b); got != want {
		t.Fatalf("Valid (len %d) %q = %v, encoding/json = %v", len(b), b, got, want)
	}
}

func TestValidLaxMatchesSpec(t *testing.T) {
	for _, doc := range laxCorpus() {
		checkLax(t, doc)
		for _, pad := range []int{laxKernelPadding, laxKernelPadding + 1, laxKernelPadding + 31, laxKernelPadding + 63} {
			checkLax(t, append([]byte(strings.Repeat(" ", pad)), doc...))
		}
	}
}

func TestValidLaxStringsAcrossBlocks(t *testing.T) {
	for shift := range 130 {
		for _, body := range []string{strings.Repeat(`\x`, 100), strings.Repeat("a\x01\\\"", 50), strings.Repeat("\\\\", 99) + "\\"} {
			doc := []byte(`[` + strings.Repeat(" ", laxKernelPadding+shift) + `"` + body + `",1]`)
			checkLax(t, doc)
		}
	}
}

func laxStringArray(shift, lengths int) []byte {
	var sb strings.Builder
	sb.WriteString("[")
	sb.WriteString(strings.Repeat(" ", laxKernelPadding+shift))
	pieces := []string{`a`, `\"`, `\\`, `\x`, "\x01", `é`, `\u00`, `,`, `]`}
	for i := range 40 {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteByte('"')
		for j := range (i*7 + lengths) % 71 {
			sb.WriteString(pieces[(i+j)%len(pieces)])
		}
		sb.WriteByte('"')
	}
	sb.WriteString("]")
	return []byte(sb.String())
}

var laxArrayShifts = []int{0, 1, 2, 3, 29, 30, 31, 32, 33, 34, 58, 59, 60, 61, 62, 63, 64, 65}

func TestValidLaxStringArrays(t *testing.T) {
	for _, shift := range laxArrayShifts {
		for lengths := range 3 {
			doc := laxStringArray(shift, lengths*13)
			checkLax(t, doc)
			for pos := laxKernelPadding + shift + lengths; pos < len(doc); pos += 11 {
				for _, c := range []byte{'x', ',', '"', '\\', ']'} {
					m := append([]byte(nil), doc...)
					m[pos] = c
					checkLax(t, m)
				}
			}
		}
	}
}

func FuzzValidLax(f *testing.F) {
	for _, doc := range laxCorpus() {
		f.Add(doc)
	}
	f.Fuzz(func(t *testing.T, b []byte) {
		checkLax(t, b)
		checkLax(t, append([]byte(strings.Repeat(" ", laxKernelPadding)), b...))
	})
}
