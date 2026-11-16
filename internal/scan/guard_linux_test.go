//go:build linux

package scan

import (
	"strings"
	"testing"

	"github.com/guno1928/alos-json/internal/guardmem"
)

func guardCorpus() []string {
	docs := []string{
		`{"id":12345,"name":"node-1","weight":1640.5,"children":[{"a":true,"b":false,"c":null}]}`,
		`[-0.25E-3,1e10,123456789012345678901234567890,0,-1,3.141592653589793,-72.98765432101234]`,
		`["` + strings.Repeat(`\u003c\"\\\n`, 30) + `"]`,
		`"` + strings.Repeat("x", 200) + `"`,
		`"` + strings.Repeat("é日本", 40) + `\uD83D\uDE00"`,
		`{"k":"` + strings.Repeat(`\u12`, 5) + `"}`,
		"[\"\x01\",\"\\x\"]",
		strings.Repeat("[", 70) + strings.Repeat("]", 70),
		` { "spaced" : [ 1 , 2 , { "x" : "y" } ] } `,
	}
	g := &docGen{x: 0x5851F42D4C957F2D}
	for range 60 {
		var sb strings.Builder
		g.value(&sb, 0)
		docs = append(docs, sb.String())
	}
	return docs
}

func exerciseScanEntryPoints(b []byte) {
	Valid(b)
	validFast(b)
	validScalar(b)
	SkipSpace(b, 0)
	FindSafeString(b)
	FindSafeStringHTML(b)
	IndexBackslash(b)
	if len(b) >= BlockLen {
		SpecialMask64(b[len(b)-BlockLen:])
	}
	for i := range min(len(b), 4) {
		NextSpecial(b, i)
		StringEnd(b, i)
		ValueEnd(b, i, 0)
		NumberEnd(b, i)
		LiteralEnd(b, i)
		AppendUnquoted(nil, b, i)
		pos := i
		escapedStringEnd(b, &pos)
	}
}

func TestScanEntryPointsNeverReadPastInput(t *testing.T) {
	for _, doc := range guardCorpus() {
		for n := 0; n <= len(doc); n++ {
			for _, piece := range []string{doc[:n], doc[len(doc)-n:]} {
				region, b, err := guardmem.Place([]byte(piece))
				if err != nil {
					t.Fatal(err)
				}
				exerciseScanEntryPoints(b)
				if err := region.Release(); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}
