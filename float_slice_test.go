package json_test

import (
	"bytes"
	stdjson "encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"

	alosjson "github.com/guno1928/alos-json"
)

const (
	floatListMarshalRounds = 400
	floatListMaxLen        = 300
	sentinelByte           = 0xEE
)

type floatListRand struct{ x uint64 }

func (r *floatListRand) next() uint64 {
	r.x ^= r.x >> 12
	r.x ^= r.x << 25
	r.x ^= r.x >> 27
	return r.x * 2685821657736338717
}

func (r *floatListRand) float() float64 {
	v := r.next()
	switch v % 7 {
	case 0:
		return math.Float64frombits(r.next())
	case 1:
		return float64(int64(r.next()%2_000_000)) / 100
	case 2:
		return -73.0 + float64(r.next()%997)*0.0123456789 + float64(r.next()%13)/1e7
	case 3:
		return float64(int64(r.next())) / math.Pow10(int(r.next()%40))
	case 4:
		return float64(int64(r.next() % 1000))
	case 5:
		return float64(r.next()>>11) * math.Pow10(int(r.next()%60)-30)
	}
	return math.Copysign(0, -1)
}

func (r *floatListRand) text() string {
	f := r.float()
	if math.IsNaN(f) || math.IsInf(f, 0) {
		f = 1.25
	}
	switch r.next() % 5 {
	case 0:
		return strconv.FormatFloat(f, 'e', int(r.next()%20), 64)
	case 1:
		return strconv.FormatFloat(f, 'f', int(r.next()%18), 64)
	case 2:
		return strconv.FormatFloat(f, 'E', -1, 64)
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

func floatListItems(r *floatListRand, n int) []string {
	items := make([]string, n)
	for i := range items {
		items[i] = r.text()
	}
	return items
}

func TestFloatSlicesLongMatchReference(t *testing.T) {
	r := &floatListRand{x: 0x7F4A7C159E3779B9}
	targets := []diffTargetFactory{
		{"floats", func() any { return new([]float64) }},
		{"floats_small_cap", func() any { s := make([]float64, 3, 5); return &s }},
		{"floats_big_cap", func() any { s := make([]float64, 2, 4096); return &s }},
		{"any", func() any { return new(any) }},
		{"any_slice", func() any { return new([]any) }},
		{"struct", func() any { return &struct{ A, B []float64 }{} }},
	}
	bad := []string{`"a,b]"`, `[1,2]`, `null`, `1.`, `-`, `1e`, `"x"`, `{}`, `true`}
	seps := []string{",", " , ", ",\n\t", "\n,"}
	for _, n := range []int{0, 1, 2, 63, 64, 65, 66, 127, 128, 129, 1000} {
		for _, sep := range seps {
			items := floatListItems(r, n)
			doc := "[" + strings.Join(items, sep) + "]"
			docs := []string{doc, " " + doc + " ", `{"A":` + doc + `,"B":` + doc + `}`, doc[:len(doc)-1], strings.TrimSuffix(doc, "]") + ",]"}
			for _, k := range []int{0, 63, 64, 65, n - 1} {
				if k < 0 || k >= n {
					continue
				}
				for _, b := range bad {
					mutated := append([]string(nil), items...)
					mutated[k] = b
					docs = append(docs, "["+strings.Join(mutated, sep)+"]")
				}
			}
			for _, d := range docs {
				for _, target := range targets {
					checkDifferential(t, d, target, nil)
				}
			}
		}
	}
}

func TestFloatTextFollowedByLongTailMatchesReference(t *testing.T) {
	r := &floatListRand{x: 0x0123456789ABCDEF}
	tail := strings.Repeat(" ", 40)
	targets := []diffTargetFactory{
		{"float", func() any { return new(float64) }},
		{"any", func() any { return new(any) }},
		{"floats", func() any { return new([]float64) }},
	}
	words := []string{"0", "-0", "01", "-01", "00.5", "0.", "-.5", "1e", "1e+", "0e-1", "1e999", "-1e999", "12345678", "123456789",
		"1234567890123456789", "12345678901234567890", "0.12345678", "0.123456789", "1.0000000000000000001", "9007199254740993",
		"123456789012345.6", "0.000000000000000000001", "4.9406564584124654e-324", "1.7976931348623159e308", "-", "1.5x", "1.5.5"}
	for range 20_000 {
		words = append(words, r.text())
	}
	for _, w := range words {
		for _, target := range targets {
			checkDifferential(t, w+tail, target, nil)
			checkDifferential(t, "["+w+","+w+"]"+tail, target, nil)
		}
	}
}

func TestMarshalFloatsMatchEncodingJSON(t *testing.T) {
	r := &floatListRand{x: 0x243F6A8885A308D3}
	for round := range floatListMarshalRounds {
		n := int(r.next() % floatListMaxLen)
		vals := make([]float64, n)
		anys := make([]any, n)
		for i := range vals {
			f := r.float()
			if math.IsNaN(f) || math.IsInf(f, 0) {
				f = float64(i)
			}
			vals[i], anys[i] = f, f
		}
		want, err := stdjson.Marshal(vals)
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range []any{vals, anys, &vals, struct{ V []float64 }{vals}} {
			got, err := alosjson.Marshal(v)
			if err != nil {
				t.Fatalf("round %d: %v", round, err)
			}
			if wantV, _ := stdjson.Marshal(v); !bytes.Equal(got, wantV) {
				t.Fatalf("round %d: Marshal(%T) = %s, want %s", round, v, got, wantV)
			}
		}
		dst := make([]byte, 3, len(want)+256)
		for i := range dst[:cap(dst)] {
			dst[:cap(dst)][i] = sentinelByte
		}
		out, err := alosjson.MarshalAppend(dst, vals)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(out[3:], want) {
			t.Fatalf("round %d: MarshalAppend = %s, want %s", round, out[3:], want)
		}
		for i, c := range out[len(out):cap(out)] {
			if c != sentinelByte {
				t.Fatalf("round %d: MarshalAppend wrote past its result at offset %d", round, len(out)+i)
			}
		}
	}
	for _, f := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		_, gotErr := alosjson.Marshal([]float64{1.5, f})
		_, wantErr := stdjson.Marshal([]float64{1.5, f})
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("Marshal([1.5, %v]) err = %v, encoding/json err = %v", f, gotErr, wantErr)
		}
		_, gotErr = alosjson.Marshal([]any{1.5, f})
		if gotErr == nil {
			t.Fatalf("Marshal([]any{1.5, %v}) succeeded", f)
		}
	}
}
