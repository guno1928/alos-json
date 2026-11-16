package scan

import (
	"strings"
	"testing"
)

func stringEndReference(b []byte, i int) (end int, escaped bool, ok bool) {
	for {
		i = NextSpecial(b, i)
		if i >= len(b) {
			return i, escaped, false
		}
		switch b[i] {
		case '"':
			return i, escaped, true
		case '\\':
			escaped = true
			if i+1 >= len(b) {
				return i, escaped, false
			}
			switch b[i+1] {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
				i += 2
			case 'u':
				if i+5 >= len(b) || !isHex[b[i+2]] || !isHex[b[i+3]] || !isHex[b[i+4]] || !isHex[b[i+5]] {
					return i, escaped, false
				}
				i += 6
			default:
				return i, escaped, false
			}
		default:
			return i, escaped, false
		}
	}
}

var stringEndPieces = []string{
	"a", "bcdefgh", `\"`, `\\`, `\/`, `\n`, `\t`, `\u00e9`, `\uD83D\uDE00`, `\u12`, `\uZZZZ`, `\x`, "\x01", "\x1f", " ",
	"é", "\xff", strings.Repeat("s", 13), strings.Repeat("t", 60), `"`, `\`,
}

func checkStringEnd(t *testing.T, b []byte) {
	t.Helper()
	for start := 0; start < len(b) && start < 3; start++ {
		gotEnd, gotEsc, gotOK := StringEnd(b, start)
		wantEnd, wantEsc, wantOK := stringEndReference(b, start)
		if gotEnd != wantEnd || gotEsc != wantEsc || gotOK != wantOK {
			t.Fatalf("StringEnd(%q, %d) = %d,%v,%v want %d,%v,%v", b, start, gotEnd, gotEsc, gotOK, wantEnd, wantEsc, wantOK)
		}
	}
}

func TestStringEndMatchesReference(t *testing.T) {
	x := uint64(0x243F6A8885A308D3)
	for range 60_000 {
		var sb strings.Builder
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		parts := 1 + int(x%40)
		for range parts {
			x ^= x >> 12
			x ^= x << 25
			x ^= x >> 27
			sb.WriteString(stringEndPieces[(x*2685821657736338717)>>58%uint64(len(stringEndPieces))])
		}
		body := sb.String()
		checkStringEnd(t, []byte(body))
		checkStringEnd(t, []byte(body+`"`+strings.Repeat("z", int(x%70))))
	}
	for pad := range 140 {
		run := strings.Repeat("p", pad)
		for _, esc := range []string{`\n`, `\u00e9`, `\uD83D\uDE00`, `\"`, `\\`} {
			doc := `\n` + run + esc + run + esc + `"` + strings.Repeat("q", 64)
			checkStringEnd(t, []byte(doc))
			checkStringEnd(t, []byte(strings.Repeat(esc, pad)+`"`+strings.Repeat("q", 80)))
		}
	}
}

func FuzzStringEnd(f *testing.F) {
	for _, p := range stringEndPieces {
		f.Add(`\n` + p + strings.Repeat(p, 30) + `"` + strings.Repeat("x", 64))
	}
	f.Fuzz(func(t *testing.T, s string) {
		checkStringEnd(t, []byte(s))
	})
}
