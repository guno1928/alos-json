package json_test

import (
	"reflect"
	"strings"
	"testing"

	alosjson "github.com/guno1928/alos-json"
)

type namedInt16 int16

type namedFloat float64

var intSliceDocs = []string{
	`[]`, `[ ]`, `[1]`, `[1,2,3]`, ` [ 1 , -2 ,\n3 ] `, "[\n1,\t2\r\n,3]", `[0,-0,00]`, `[01]`, `[-]`, `[1,]`, `[,1]`, `[1 2]`, `[1,2`, `[1,2,`,
	`[1,null,3]`, `[null]`, `["7",8]`, `[1,"x"]`, `[3.9,4]`, `[1e2]`, `[1,2.5e1]`, `[127,128]`, `[-128,-129]`, `[32767,32768]`,
	`[2147483647,2147483648]`, `[9223372036854775807]`, `[9223372036854775808]`, `[-9223372036854775808]`,
	`[123456789012345678,1234567890123456789]`, `[true]`, `[{}]`, `[[1]]`, `null`, `{}`, `"x"`, `[1]x`, `[1]]`,
	`[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]`,
	`[1.5,-2e3,0.1,-0.0]`, `[1e400]`, `[-1e400]`, `[1e-400]`, `[12345678901234567890123.5]`, `["1.5"]`, `[1.]`, `[.5]`, `[1.5e]`,
	`[0.30000000000000004,1.7976931348623157e308,4.9e-324]`, `[-72.98765432109877,139.6917]`, `[1E+2,1e-2]`,
	`["a"]`, `["a","b" , "c"]`, `[ "x" ,"y"]`, `["esc\n","\u00e9\uD83D\uDE00"]`, `["a",1]`, `["a",null]`, `["bad\x"]`, `["unterminated]`,
	`["","",""]`, "[\"ctl\x01\"]", `["a","b"`, `["\"q\"","\\"]`, `["` + strings.Repeat("long", 300) + `","b"]`,
}

func intSliceTargets() []func() any {
	return []func() any{
		func() any { return new([]int) },
		func() any { return new([]int8) },
		func() any { return new([]int16) },
		func() any { return new([]int32) },
		func() any { return new([]int64) },
		func() any { return new([]namedInt16) },
		func() any { return new([][]int) },
		func() any { s := []int{9, 9, 9, 9, 9, 9, 9, 9}; return &s },
		func() any { s := make([]int64, 2, 3); s[0], s[1] = 5, 6; return &s },
		func() any { s := []int8{}; return &s },
		func() any { return &struct{ A, B []int32 }{A: []int32{1, 2, 3, 4}} },
		func() any { return new([]float64) },
		func() any { return new([]namedFloat) },
		func() any { s := []float64{7, 7, 7, 7, 7}; return &s },
		func() any { return &struct{ A, B []float64 }{B: []float64{1}} },
		func() any { return new([]string) },
		func() any { s := []string{"keep", "keep", "keep", "keep", "keep"}; return &s },
		func() any { return &struct{ A, B []string }{A: []string{"x"}} },
	}
}

func TestIntegerSlicesMatchReference(t *testing.T) {
	docs := append([]string(nil), intSliceDocs...)
	for _, d := range intSliceDocs {
		docs = append(docs, `{"A":`+d+`,"B":`+d+`}`)
	}
	for _, doc := range docs {
		for i, mk := range intSliceTargets() {
			got, want := mk(), mk()
			gotErr := alosjson.Unmarshal([]byte(doc), got)
			wantErr := alosjson.UnmarshalReference([]byte(doc), want)
			if (gotErr == nil) != (wantErr == nil) {
				t.Fatalf("doc %q target %d: err %v, reference %v", doc, i, gotErr, wantErr)
			}
			if wantErr != nil {
				continue
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("doc %q target %d: got %#v, reference %#v", doc, i, got, want)
			}
			if gv, wv := reflect.ValueOf(got).Elem(), reflect.ValueOf(want).Elem(); gv.Kind() == reflect.Slice && !gv.IsNil() && gv.Len() < gv.Cap() {
				shared := min(gv.Cap(), wv.Cap())
				full, wantFull := gv.Slice(0, shared), wv.Slice(0, shared)
				if !reflect.DeepEqual(full.Interface(), wantFull.Interface()) {
					t.Fatalf("doc %q target %d: spare capacity differs %v vs %v", doc, i, full, wantFull)
				}
			}
		}
	}
}
