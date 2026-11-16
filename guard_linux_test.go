//go:build linux

package json_test

import (
	"strings"
	"testing"
	"unsafe"

	alosjson "github.com/guno1928/alos-json"
	"github.com/guno1928/alos-json/internal/fnum"
	"github.com/guno1928/alos-json/internal/guardmem"
	"github.com/guno1928/alos-json/jsontext"
)

type guardRecord struct {
	ID       int64          `json:"id"`
	Name     string         `json:"name"`
	Weight   float64        `json:"weight"`
	Tags     []string       `json:"tags"`
	Ints     []int64        `json:"ints"`
	Floats   []float64      `json:"floats"`
	Children []*guardRecord `json:"children"`
	Extra    map[string]any `json:"extra"`
	Flag     *bool          `json:"flag"`
}

func guardDocs() []string {
	docs := []string{
		`{"id":12345,"name":"node-1","weight":1640.5,"tags":["a","b\n","` + strings.Repeat("t", 90) + `"],"ints":[1,-2,30000000000],"floats":[0.5,-1e-7,3.141592653589793],"children":[{"id":2,"children":[]}],"extra":{"k":[null,true,{"z":"é"}]},"flag":false}`,
		`[` + strings.Repeat(`"<\"\\\n",`, 20) + `1.5e300,-0,123456789012345678901234567890]`,
		`{"name":"` + strings.Repeat("é日本😀", 30) + `","ints":[` + strings.Repeat("9007199254740993,", 10) + `0]}`,
		`{"floats":[` + strings.Repeat("-72.98765432101234,", 12) + `1]}`,
		`{"name":"` + strings.Repeat(`\uD83D\uDE00`, 12) + `"}`,
		strings.Repeat(`[{"id":1,"children":`, 40) + "null" + strings.Repeat("}]", 40),
	}
	g := &validDocGen{x: 0xDA942042E4DD58B5}
	for range 40 {
		var sb strings.Builder
		g.value(&sb, 0)
		docs = append(docs, sb.String())
	}
	return docs
}

func exercisePublicReaders(b []byte) {
	alosjson.Valid(b)
	jsontext.Value(b).IsValid()
	var a any
	alosjson.Unmarshal(b, &a)
	var m map[string]any
	alosjson.Unmarshal(b, &m)
	var s []any
	alosjson.Unmarshal(b, &s)
	var r guardRecord
	alosjson.Unmarshal(b, &r)
	var rs []guardRecord
	alosjson.Unmarshal(b, &rs)
	var ints []int64
	alosjson.Unmarshal(b, &ints)
	var floats []float64
	alosjson.Unmarshal(b, &floats)
	var strs []string
	alosjson.Unmarshal(b, &strs)
	var str string
	alosjson.Unmarshal(b, &str)
	jsontext.AppendUnquote(nil, b)
	jsontext.FindSafeStringBytes(b)
	jsontext.FindSafeStringBytesHTML(b)
	if len(b) > 0 {
		text := unsafe.String(&b[0], len(b))
		fnum.ParseFloat64(text)
		jsontext.AppendQuoteString(nil, text)
		jsontext.AppendQuoteStringHTML(nil, text)
		alosjson.Marshal(text)
		alosjson.Marshal(map[string]string{text: text})
	}
}

func TestPublicReadersNeverReadPastInput(t *testing.T) {
	for _, doc := range guardDocs() {
		for n := 0; n <= len(doc); n++ {
			for _, piece := range []string{doc[:n], doc[len(doc)-n:]} {
				region, b, err := guardmem.Place([]byte(piece))
				if err != nil {
					t.Fatal(err)
				}
				exercisePublicReaders(b)
				if err := region.Release(); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
}

func TestNumberPrefixesNeverReadPastInput(t *testing.T) {
	numbers := []string{"-0.000123456789012345e-300", "123456789012345678901234567890.5e+10", "18446744073709551616", "-9223372036854775809", "1.7976931348623157e308", "4.9e-324"}
	for _, num := range numbers {
		for n := 1; n <= len(num); n++ {
			region, b, err := guardmem.Place([]byte(num[:n]))
			if err != nil {
				t.Fatal(err)
			}
			fnum.ParseFloat64(unsafe.String(&b[0], len(b)))
			exercisePublicReaders(b)
			if err := region.Release(); err != nil {
				t.Fatal(err)
			}
		}
	}
}
