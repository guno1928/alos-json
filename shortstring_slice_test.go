package json

import (
	stdjson "encoding/json"
	"reflect"
	"strings"
	"testing"
)

const (
	shortSliceMaxLen  = 20
	shortSliceRounds  = 20_000
	shortSliceMaxElem = 12
)

var shortSliceAlphabet = []string{"a", "Z", "9", " ", "\\\"", "\\\\", "\\n", "\\u00e9", "é", "\t", "\x01", "\"", ","}

func TestStringSliceShortElementsAtWindowEdges(t *testing.T) {
	for n := range shortSliceMaxLen + 1 {
		for _, tail := range []string{"", ",", "]", " ", "\"x\"]"} {
			word := strings.Repeat("w", n)
			inputs := []string{
				`["` + word + `"]`,
				`["` + word + `","` + word + `"]` + tail,
				`["` + word + `" , "` + word + `"]`,
				`["` + word + `\n"]`,
				`["` + word + "\x1f" + `"]`,
				`["` + word + `"`,
				`["` + word,
			}
			for _, in := range inputs {
				compareStringSliceDecode(t, in)
			}
		}
	}
}

func TestStringSliceShortElementsRandom(t *testing.T) {
	x := uint64(0x2545F4914F6CDD1D)
	next := func() uint64 {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		return x * 2685821657736338717
	}
	for range shortSliceRounds {
		var sb strings.Builder
		sb.WriteByte('[')
		elems := int(next() % shortSliceMaxElem)
		for e := range elems {
			if e > 0 {
				sb.WriteByte(',')
			}
			sb.WriteByte('"')
			for range int(next() % shortSliceMaxLen) {
				sb.WriteString(shortSliceAlphabet[next()%uint64(len(shortSliceAlphabet))])
			}
			sb.WriteByte('"')
		}
		sb.WriteByte(']')
		compareStringSliceDecode(t, sb.String())
	}
}

func compareStringSliceDecode(t *testing.T, in string) {
	t.Helper()
	var got, want []string
	gotErr := Unmarshal([]byte(in), &got)
	wantErr := stdjson.Unmarshal([]byte(in), &want)
	if (gotErr == nil) != (wantErr == nil) {
		t.Fatalf("%q: error mismatch: got %v, std %v", in, gotErr, wantErr)
	}
	if gotErr == nil && !reflect.DeepEqual(got, want) {
		t.Fatalf("%q: got %q, std %q", in, got, want)
	}
}
