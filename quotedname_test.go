package json

import (
	"strings"
	"testing"
)

func TestQuotedNamePrefixOfMatchesHasPrefix(t *testing.T) {
	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	for n := 0; n <= 20; n++ {
		literal := ""
		if n > 0 {
			literal = `"` + alphabet[:max(n-2, 0)] + `"`
			literal = literal[:n]
		}
		q := newQuotedName(literal)
		for _, tail := range []string{"", ":", ":1}", strings.Repeat("x", 24)} {
			buffers := []string{literal + tail}
			for i := range literal {
				mutated := []byte(literal + tail)
				mutated[i] ^= 0x20
				buffers = append(buffers, string(mutated))
			}
			for cut := 0; cut < len(literal); cut++ {
				buffers = append(buffers, literal[:cut])
			}
			for _, buf := range buffers {
				if got, want := q.prefixOf([]byte(buf)), hasPrefix([]byte(buf), literal); got != want {
					t.Fatalf("literal %q buf %q: prefixOf=%v hasPrefix=%v", literal, buf, got, want)
				}
			}
		}
	}
}
