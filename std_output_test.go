package json_test

import (
	"bytes"
	stdjson "encoding/json"
	"math"
	"strings"
	"testing"
	"unicode/utf8"

	alosjson "github.com/guno1928/alos-json"
)

func checkSameOutputAsStd(t *testing.T, v any, opts ...alosjson.Option) {
	t.Helper()
	want, err := stdjson.Marshal(v)
	if err != nil {
		t.Fatalf("encoding/json: %v", err)
	}
	got, err := alosjson.Marshal(v, opts...)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("Marshal(%#v) = %s (err %v), encoding/json = %s", v, got, err, want)
	}
}

func floatSamples() []float64 {
	out := []float64{0, math.Copysign(0, -1), 1e-6, 1e21, 1e-7, 1.5e-9, 123e-10, 9.999999999999999e20, 5e-324, math.MaxFloat64, math.SmallestNonzeroFloat64, 1e300, -1e300, 0.1, 98.5, 1e15}
	for _, cut := range []float64{1e-6, 1e21} {
		out = append(out, math.Nextafter(cut, 0), math.Nextafter(cut, math.Inf(1)))
	}
	x := uint64(0xC2B2AE3D27D4EB4F)
	for range 300_000 {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		if f := math.Float64frombits(x * 2685821657736338717); !math.IsNaN(f) && !math.IsInf(f, 0) {
			out = append(out, f)
		}
	}
	return out
}

func TestFloatOutputMatchesEncodingJSON(t *testing.T) {
	for _, f := range floatSamples() {
		checkSameOutputAsStd(t, f)
		if f32 := float32(f); !math.IsInf(float64(f32), 0) {
			checkSameOutputAsStd(t, f32)
		}
	}
	checkSameOutputAsStd(t, []float64{1e-7, 2.5e30, -4e-300})
	checkSameOutputAsStd(t, map[string]any{"a": 1e25})
}

var htmlPieces = []string{"\xe2\x80\xa8", "\xe2\x80\xa9", "\xe2\x80", "\xe2\x80\xa7", "\xe2\x80\xaa", "\xe2", "<", ">", "&", "a", "é", "日本", "😀", "\"", "\\", "\n", "\t", "\x00", "\x1f", "\x7f", strings.Repeat("s", 40)}

func TestEscapeHTMLMatchesEncodingJSON(t *testing.T) {
	x := uint64(0x165667B19E3779F9)
	checked := 0
	for range 50_000 {
		var sb strings.Builder
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		for n := x % 24; n > 0; n-- {
			x ^= x >> 12
			x ^= x << 25
			x ^= x >> 27
			sb.WriteString(htmlPieces[x%uint64(len(htmlPieces))])
		}
		s := sb.String()
		if !utf8.ValidString(s) {
			continue
		}
		checkSameOutputAsStd(t, s, alosjson.EscapeHTML(true))
		checkSameOutputAsStd(t, map[string]string{s: s}, alosjson.EscapeHTML(true))
		checked++
	}
	if checked < 10_000 {
		t.Fatalf("only %d strings checked", checked)
	}
}

func TestLineSeparatorsRawWithoutEscapeHTML(t *testing.T) {
	got, err := alosjson.Marshal("a\xe2\x80\xa8b\xe2\x80\xa9c<&>")
	if err != nil || string(got) != "\"a\xe2\x80\xa8b\xe2\x80\xa9c<&>\"" {
		t.Fatalf("Marshal without EscapeHTML = %q (err %v)", got, err)
	}
}
