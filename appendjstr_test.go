package json

import (
	"bytes"
	"testing"

	"github.com/guno1928/alos-json/jsontext"
)

func TestAppendJStrMatchesIndependentQuoter(t *testing.T) {
	prefixes := [][]byte{nil, []byte("pre"), make([]byte, 5, 8)}
	for n := 0; n <= 40; n++ {
		base := bytes.Repeat([]byte{'a'}, n)
		check := func(s string) {
			for _, prefix := range prefixes {
				dst := append(prefix[:len(prefix):len(prefix)], []byte{}...)
				got := appendJStr(dst, s)
				want := jsontext.AppendQuote(append([]byte(nil), prefix...), s)
				if !bytes.Equal(got, want) {
					t.Fatalf("appendJStr(%q) = %q, want %q", s, got, want)
				}
			}
		}
		check(string(base))
		for pos := range n {
			for c := range 256 {
				b := append([]byte(nil), base...)
				b[pos] = byte(c)
				check(string(b))
			}
		}
	}
}
