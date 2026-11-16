package scan

import (
	"encoding/json"
	"strings"
	"testing"
)

func xorshiftBytes(seed uint64) func() uint64 {
	x := seed
	return func() uint64 {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		return x
	}
}

var validEscapePieces = []string{`\n`, `\"`, `\\`, `\/`, `\b`, `\f`, `\r`, `\t`, `\u00e9`, `\uD83D\uDE00`, `\u003c`, `\uABCD`, "a", "é", "日", "x y", `\\u0041`, `\\\"`, `\\\\`}

var invalidEscapePieces = []string{`\`, `\u12`, `\u12g4`, `\x`, "\x01", `\U0041`}

const invalidPieceOdds = 200

func randomEscapedString(next func() uint64) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for range 1 + next()%40 {
		if next()%5 == 0 {
			sb.WriteString(strings.Repeat("p", int(next()%70)))
		}
		if next()%invalidPieceOdds == 0 {
			sb.WriteString(invalidEscapePieces[next()%uint64(len(invalidEscapePieces))])
		} else {
			sb.WriteString(validEscapePieces[next()%uint64(len(validEscapePieces))])
		}
	}
	sb.WriteByte('"')
	return sb.String()
}

func randomEscapedDoc(next func() uint64) []byte {
	var sb strings.Builder
	sb.WriteString(strings.Repeat(" ", int(next()%9)))
	sb.WriteByte('[')
	for k := range 1 + next()%6 {
		if k > 0 {
			sb.WriteByte(',')
		}
		if next()%2 == 0 {
			sb.WriteString("{" + randomEscapedString(next) + ":" + randomEscapedString(next) + "}")
		} else {
			sb.WriteString(randomEscapedString(next))
		}
	}
	sb.WriteByte(']')
	return []byte(sb.String())
}

func checkValidAgainstStdlib(t *testing.T, doc []byte) {
	t.Helper()
	checkValid(t, doc)
	if got, want := validFast(doc), json.Valid(doc); got != want {
		t.Fatalf("validFast(%q) = %v, encoding/json.Valid = %v", doc, got, want)
	}
}

func TestValidFastLongEscapedStrings(t *testing.T) {
	next := xorshiftBytes(0x9E3779B97F4A7C15)
	valid := 0
	for range 4_000 {
		doc := randomEscapedDoc(next)
		checkValidAgainstStdlib(t, doc)
		if json.Valid(doc) {
			valid++
		}
		for cut := max(1, len(doc)-70); cut < len(doc); cut++ {
			checkValidAgainstStdlib(t, doc[:cut])
		}
		for start := 1; start < min(len(doc), 70); start++ {
			checkValidAgainstStdlib(t, doc[start:])
		}
	}
	if valid < 1_600 {
		t.Fatalf("generator produced only %d valid documents", valid)
	}
}

func FuzzValidFastEscapedStrings(f *testing.F) {
	next := xorshiftBytes(0x2545F4914F6CDD1D)
	for range 40 {
		f.Add(randomEscapedDoc(next))
	}
	f.Fuzz(func(t *testing.T, doc []byte) {
		checkValidAgainstStdlib(t, doc)
	})
}
