package scan_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/guno1928/alos-json/internal/scan"
	"github.com/guno1928/alos-json/jsontext"
)

var unquotePieces = []string{
	"a", "plain text ", "é", "日本", "😀", "\x80", "\xff", "\"", "\\", "\x00", "\x1f", "\t", " ",
	`\"`, `\`, `\/`, `\b`, `\f`, `\n`, `\r`, `\t`, `\u0041`, `\u00e9`, `\u65e5`, `\uD83D\uDE00`,
	`\uD83D`, `\uDE00`, `\uD83D\u0041`, `\uD83D\uD83D`, `\uDBFF\uDFFF`, `\uZZZZ`, `\u12`, `\x`, `\`,
	`\uD83D\uZZZZ`, `\uD83D\`, `\U0041`, strings.Repeat("x", 31), strings.Repeat("y", 64),
}

func checkAppendUnquoted(t *testing.T, body string) {
	t.Helper()
	b := []byte(`"` + body)
	gotDst, gotEnd, gotOK := scan.AppendUnquoted([]byte("pre"), b, 1)
	wantEnd, _, wantOK := scan.StringEnd(b, 1)
	if gotEnd != wantEnd || gotOK != wantOK {
		t.Fatalf("AppendUnquoted(%q) end=%d ok=%v, StringEnd end=%d ok=%v", b, gotEnd, gotOK, wantEnd, wantOK)
	}
	if !gotOK {
		return
	}
	want := jsontext.AppendUnquote([]byte("pre"), b[:wantEnd+1])
	if !bytes.Equal(gotDst, want) {
		t.Fatalf("AppendUnquoted(%q) = %q, want %q", b, gotDst, want)
	}
}

func TestAppendUnquotedMatchesReference(t *testing.T) {
	n := len(unquotePieces)
	for a := range n {
		for b := range n {
			for c := range n {
				body := unquotePieces[a] + unquotePieces[b] + unquotePieces[c]
				checkAppendUnquoted(t, body)
				checkAppendUnquoted(t, body+`"`)
				checkAppendUnquoted(t, body+`"tail`)
			}
		}
	}
}

func TestAppendUnquotedLongRuns(t *testing.T) {
	for n := range 80 {
		run := strings.Repeat("r", n)
		for _, esc := range []string{`\n`, `\u00e9`, `\uD83D\uDE00`, "\x01", `"`, `\`} {
			checkAppendUnquoted(t, run+esc+run+`"`)
			checkAppendUnquoted(t, esc+run+esc+`"`)
		}
	}
}

func TestAppendUnquotedLongEscapeMixes(t *testing.T) {
	x := uint64(0x13198A2E03707344)
	for range 40_000 {
		var sb strings.Builder
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		for range 10 + int(x%60) {
			x ^= x >> 12
			x ^= x << 25
			x ^= x >> 27
			sb.WriteString(unquotePieces[(x*2685821657736338717)>>58%uint64(len(unquotePieces))])
		}
		body := sb.String()
		checkAppendUnquoted(t, body)
		checkAppendUnquoted(t, `\n`+body+`"`+strings.Repeat("z", 70))
	}
}

func FuzzAppendUnquoted(f *testing.F) {
	for _, p := range unquotePieces {
		f.Add(p + `"`)
	}
	f.Fuzz(func(t *testing.T, body string) {
		checkAppendUnquoted(t, body)
	})
}
