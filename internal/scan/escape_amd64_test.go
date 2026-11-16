//go:build amd64 && !purego

package scan

import (
	"strings"
	"testing"
)

func checkEscapedBlocks(t *testing.T, b []byte, start int) {
	t.Helper()
	if !useEscapeKernel() {
		return
	}
	pos := start
	gotEnd, done, gotOK := escapedStringEnd(b, &pos)
	wantEnd, wantOK := scalarStringEnd(b, start)
	if !done || gotEnd != wantEnd || gotOK != wantOK {
		t.Fatalf("escapedStringEnd(%q, %d) = %d,%v,%v want %d,%v", b, start, gotEnd, done, gotOK, wantEnd, wantOK)
	}
}

var escapePieces = []string{`\n`, `\"`, `\\`, `\/`, `\b`, `\f`, `\r`, `\t`, `\u00e9`, `\uD83D\uDE00`, `\u003c`, "a", "é", "日", `\`, `\u12`, `\u12g4`, `\x`, "\x01", `"`, `\\"`, `\\\"`}

func TestEscapedBlocksMatchScalar(t *testing.T) {
	x := uint64(0xA54FF53A5F1D36F1)
	next := func() uint64 {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		return x
	}
	for range 40_000 {
		var sb strings.Builder
		for range 1 + next()%60 {
			if next()%4 == 0 {
				sb.WriteString(strings.Repeat("p", int(next()%70)))
			}
			sb.WriteString(escapePieces[next()%uint64(len(escapePieces))])
		}
		if next()%3 != 0 {
			sb.WriteByte('"')
		}
		sb.WriteString(strings.Repeat("z", int(next()%80)))
		b := []byte(sb.String())
		for start := 0; start < len(b) && start < 4; start++ {
			checkEscapedBlocks(t, b, start)
		}
	}
}

func TestEscapedBlocksUnicodeAcrossBoundary(t *testing.T) {
	tails := []string{`\u0041"`, `\u004"`, `\u00G1"`, `\uABCD\uabcd"`, `\\u0041"`, `\\\u0041"`, `\u0041`, `\u00`, `\`, `\"`, `\""`}
	for lead := range 3 * BlockLen {
		for _, tail := range tails {
			for _, after := range []string{"", strings.Repeat("y", 70)} {
				b := []byte(strings.Repeat("q", lead) + tail + after)
				checkEscapedBlocks(t, b, 0)
				checkEscapedBlocks(t, append([]byte(`\n`), b...), 0)
			}
		}
	}
}

func FuzzEscapedBlocks(f *testing.F) {
	for _, p := range escapePieces {
		f.Add(strings.Repeat(p, 40) + `"` + strings.Repeat("x", 64))
	}
	f.Fuzz(func(t *testing.T, s string) {
		checkEscapedBlocks(t, []byte(s), 0)
	})
}

func TestEscapedKernelClassifiesEveryByte(t *testing.T) {
	contexts := []string{"", `\`, `\u`, `\u0`, `\u00`, `\u000`, `\\`, `\\\`}
	for c := range 256 {
		for _, ctx := range contexts {
			for pos := 0; pos < 2*BlockLen+8; pos += 3 {
				for _, after := range []string{`"`, `"` + strings.Repeat("y", 70)} {
					b := []byte(strings.Repeat("q", pos) + ctx + string([]byte{byte(c)}) + "zz" + after)
					checkEscapedBlocks(t, b, 0)
					checkEscapedBlocks(t, b, pos)
				}
			}
		}
	}
}

func TestEscapedKernelBackslashRuns(t *testing.T) {
	alphabet := []byte{'\\', '\\', '\\', 'n', 'a', '"', 'u', '0', 'F', 0x01}
	next := xorshiftBytes(0x243F6A8885A308D3)
	for range 60_000 {
		b := make([]byte, BlockLen+int(next()%(4*BlockLen)))
		for i := range b {
			b[i] = alphabet[next()%uint64(len(alphabet))]
		}
		for start := range 4 {
			checkEscapedBlocks(t, b, start)
		}
	}
}

func TestEscapedKernelTruncatedAtEveryLength(t *testing.T) {
	var sb strings.Builder
	for i, p := range escapePieces {
		if p == `\` || p == `"` || p == `\\"` || p == `\\\"` || p == "\x01" || p == `\u12` || p == `\u12g4` || p == `\x` {
			continue
		}
		sb.WriteString(p)
		sb.WriteString(strings.Repeat("w", i%5))
	}
	body := sb.String()
	doc := []byte(body + body + body + `"` + strings.Repeat("t", 40))
	for n := 0; n <= len(doc); n++ {
		for start := range 4 {
			checkEscapedBlocks(t, doc[:n], min(start, n))
		}
	}
}
