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

func checkAppendUnquotedInto(t *testing.T, dst []byte, body string) {
	t.Helper()
	b := []byte(`"` + body)
	prefix := string(dst)
	gotDst, gotEnd, gotOK := scan.AppendUnquoted(dst, b, 1)
	wantEnd, _, wantOK := scan.StringEnd(b, 1)
	if gotEnd != wantEnd || gotOK != wantOK {
		t.Fatalf("AppendUnquoted(%q) cap=%d end=%d ok=%v, StringEnd end=%d ok=%v", b, cap(dst), gotEnd, gotOK, wantEnd, wantOK)
	}
	if !gotOK {
		return
	}
	want := jsontext.AppendUnquote([]byte(prefix), b[:wantEnd+1])
	if !bytes.Equal(gotDst, want) {
		t.Fatalf("AppendUnquoted(%q) cap=%d = %q, want %q", b, cap(dst), gotDst, want)
	}
}

var unquoteSpareCaps = []int{0, 1, 2, 31, 32, 33, 63, 64, 65, 66, 95, 96, 97, 130, 300}

func TestAppendUnquotedEscapeAtEveryOffset(t *testing.T) {
	tail := strings.Repeat("r", 70) + `"` + strings.Repeat("z", 70)
	for lead := range 140 {
		for _, piece := range unquotePieces {
			body := strings.Repeat("q", lead) + piece + tail
			for _, spare := range unquoteSpareCaps {
				dst := append(make([]byte, 0, 3+spare), "pre"...)
				checkAppendUnquotedInto(t, dst, body)
			}
		}
	}
}

func TestAppendUnquotedTruncatedAtEveryLength(t *testing.T) {
	var sb strings.Builder
	for i, p := range unquotePieces {
		sb.WriteString(p)
		sb.WriteString(strings.Repeat("w", i%7))
	}
	full := sb.String() + sb.String() + `"` + strings.Repeat("t", 40)
	for n := 0; n <= len(full); n++ {
		for _, spare := range []int{0, 64, 200} {
			checkAppendUnquotedInto(t, make([]byte, 0, spare), full[:n])
		}
	}
}

func TestAppendUnquotedRandomMixesWithSpareCapacity(t *testing.T) {
	x := uint64(0x452821E638D01377)
	next := func() uint64 {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		return x
	}
	for range 20_000 {
		var sb strings.Builder
		for range 5 + next()%80 {
			sb.WriteString(unquotePieces[next()%uint64(len(unquotePieces))])
		}
		spare := unquoteSpareCaps[next()%uint64(len(unquoteSpareCaps))]
		body := sb.String() + `"` + strings.Repeat("z", int(next()%90))
		checkAppendUnquotedInto(t, make([]byte, 0, spare), body)
	}
}
