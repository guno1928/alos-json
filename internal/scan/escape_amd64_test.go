//go:build amd64 && !purego

package scan

import (
	"strings"
	"testing"
)

func escapeBlockReference(b []byte) escapeBlock {
	var m escapeBlock
	for i := range BlockLen {
		bit := uint64(1) << i
		c := b[i]
		if c == '"' {
			m.quote |= bit
		}
		if c == '\\' {
			m.bslash |= bit
		}
		if c < 0x20 {
			m.ctrl |= bit
		}
		if !strings.ContainsRune(`"\/bfnrt`, rune(c)) || c >= 0x80 {
			m.notSimple |= bit
		}
		if c != 'u' {
			m.notU |= bit
		}
		if !isHex[c] {
			m.notHex |= bit
		}
	}
	return m
}

func TestEscapeMasksMatchReference(t *testing.T) {
	if !useAVX2 {
		t.Skip("no AVX2")
	}
	buf := make([]byte, BlockLen)
	var got escapeBlock
	check := func() {
		t.Helper()
		escapeMasksAVX2(&buf[0], &got)
		if want := escapeBlockReference(buf); got != want {
			t.Fatalf("escapeMasksAVX2(%q) = %+v, want %+v", buf, got, want)
		}
	}
	for c := range 256 {
		for pos := range BlockLen {
			for i := range buf {
				buf[i] = 'x'
			}
			buf[pos] = byte(c)
			buf[(pos*13+7)%BlockLen] = byte(c)
			check()
		}
	}
	x := uint64(0x3C6EF372FE94F82B)
	for range 50_000 {
		for i := range buf {
			x ^= x >> 12
			x ^= x << 25
			x ^= x >> 27
			buf[i] = byte(x >> 56)
		}
		check()
	}
}

func checkEscapedBlocks(t *testing.T, b []byte, start int) {
	t.Helper()
	if !useAVX2 {
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

func escapedReference(b []byte) []bool {
	out := make([]bool, len(b))
	run := 0
	for i, c := range b {
		out[i] = run%2 == 1
		if c == '\\' {
			run++
		} else {
			run = 0
		}
	}
	return out
}

func TestEscapedMaskMatchesReference(t *testing.T) {
	x := uint64(0x243F6A8885A308D3)
	for range 20_000 {
		n := BlockLen * (1 + int(x%4))
		b := make([]byte, n)
		for i := range b {
			x ^= x >> 12
			x ^= x << 25
			x ^= x >> 27
			if x%3 == 0 {
				b[i] = 'a'
			} else {
				b[i] = '\\'
			}
		}
		want := escapedReference(b)
		var prev uint64
		for blk := 0; blk < n; blk += BlockLen {
			var bs uint64
			for i := range BlockLen {
				if b[blk+i] == '\\' {
					bs |= 1 << i
				}
			}
			got := escapedMask(bs, &prev)
			for i := range BlockLen {
				if (got>>i&1 == 1) != want[blk+i] {
					t.Fatalf("escaped bit %d of %q: got %v want %v", blk+i, b, got>>i&1 == 1, want[blk+i])
				}
			}
		}
	}
}
