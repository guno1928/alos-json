package json_test

import (
	"bytes"
	"errors"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
	"testing/iotest"

	alosjson "github.com/guno1928/alos-json"
	"github.com/guno1928/alos-json/jsontext"
)

var malformedDocuments = []string{
	`[1 2]`,
	`[1,,2]`,
	`[,1]`,
	`[1,]`,
	`[,]`,
	`{"a" 1}`,
	`{"a"::1}`,
	`{"a":1,}`,
	`{,"a":1}`,
	`{"a":1 "b":2}`,
	`{"a",1}`,
	`{1:2}`,
	`{"a":1,,"b":2}`,
	`{"a":}`,
	`[1]]`,
	`{"a":1}}`,
	`{"a":1} xyz`,
	`[1] [2]`,
	`1 2`,
	`"a" "b"`,
	`[1]x`,
	`[1:2]`,
	`{"a":[1,2,]}`,
	`[{"a":1},]`,
	`[[1] [2]]`,
}

type sepTarget struct {
	A any `json:"a"`
	B any `json:"b"`
}

func TestRejectMalformedSeparators(t *testing.T) {
	for _, in := range malformedDocuments {
		var generic any
		if err := alosjson.Unmarshal([]byte(in), &generic); err == nil {
			t.Errorf("Unmarshal(any) accepted %q -> %v", in, generic)
		}
		var typed sepTarget
		if err := alosjson.Unmarshal([]byte(in), &typed); err == nil {
			t.Errorf("Unmarshal(struct) accepted %q", in)
		}
		var streamed any
		if err := alosjson.UnmarshalRead(strings.NewReader(in), &streamed); err == nil {
			t.Errorf("UnmarshalRead accepted %q", in)
		}
		if alosjson.Valid([]byte(in)) {
			t.Errorf("Valid accepted %q", in)
		}
	}
}

func TestAcceptWellFormed(t *testing.T) {
	docs := []string{
		`[]`, `{}`, `[1]`, `[1,2,3]`, ` [ 1 , 2 ] `, "\n{\n\t\"a\" : 1 ,\r\n \"b\":[ ]}\n",
		`{"a":{"b":{"c":[1,{"d":null}]}}}`, `"x"`, `0`, `-0.5e+10`, `true`, `null`,
	}
	for _, in := range docs {
		var generic any
		if err := alosjson.Unmarshal([]byte(in), &generic); err != nil {
			t.Errorf("Unmarshal rejected %q: %v", in, err)
		}
		var streamed any
		if err := alosjson.UnmarshalRead(strings.NewReader(in), &streamed); err != nil {
			t.Errorf("UnmarshalRead rejected %q: %v", in, err)
		}
		if !reflect.DeepEqual(generic, streamed) {
			t.Errorf("Unmarshal and UnmarshalRead disagree on %q: %v vs %v", in, generic, streamed)
		}
		if !alosjson.Valid([]byte(in)) {
			t.Errorf("Valid rejected %q", in)
		}
	}
}

func nested(depth int) []byte {
	return []byte(strings.Repeat("[", depth) + strings.Repeat("]", depth))
}

func TestNestingDepthLimit(t *testing.T) {
	var ok any
	if err := alosjson.Unmarshal(nested(1000), &ok); err != nil {
		t.Fatalf("depth 1000 rejected: %v", err)
	}
	for _, depth := range []int{10_001, 1_000_000} {
		var v any
		if err := alosjson.Unmarshal(nested(depth), &v); err == nil {
			t.Errorf("depth %d accepted by Unmarshal", depth)
		}
		if err := alosjson.UnmarshalRead(bytes.NewReader(nested(depth)), &v); err == nil {
			t.Errorf("depth %d accepted by UnmarshalRead", depth)
		}
		if alosjson.Valid(nested(depth)) {
			t.Errorf("depth %d accepted by Valid", depth)
		}
	}
}

func TestMarshalRejectsNaNInf(t *testing.T) {
	type F64 struct{ X float64 }
	type F32 struct{ X float32 }
	type Mixed struct {
		S string
		X float64
	}
	bad := []float64{math.NaN(), math.Inf(1), math.Inf(-1)}
	for _, f := range bad {
		values := []any{
			f,
			float32(f),
			F64{X: f},
			&F64{X: f},
			F32{X: float32(f)},
			Mixed{S: "s", X: f},
			[]float64{1, f},
			map[string]float64{"k": f},
			map[string]any{"k": f},
			[]any{f},
			struct{ V any }{V: f},
		}
		for _, v := range values {
			if out, err := alosjson.Marshal(v); err == nil {
				t.Errorf("Marshal(%T with %v) succeeded: %s", v, f, out)
			}
		}
	}
}

func TestUnmarshalNumberRanges(t *testing.T) {
	type I8 struct{ N int8 }
	type U8 struct{ N uint8 }
	type U struct{ N uint }
	type I64 struct{ N int64 }
	type U64 struct{ N uint64 }
	type I struct{ N int }
	failing := []struct {
		in  string
		dst any
	}{
		{`{"N":300}`, new(I8)},
		{`{"N":-129}`, new(I8)},
		{`{"N":256}`, new(U8)},
		{`{"N":-1}`, new(U8)},
		{`{"N":-1}`, new(U)},
		{`{"N":9223372036854775808}`, new(I64)},
		{`{"N":-9223372036854775809}`, new(I64)},
		{`{"N":18446744073709551616}`, new(U64)},
		{`{"N":1e30}`, new(I)},
		{`{"N":"300"}`, new(I8)},
		{`[300]`, new([]int8)},
		{`300`, new(int8)},
		{`{"k":300}`, new(map[string]int8)},
	}
	for _, tc := range failing {
		if err := alosjson.Unmarshal([]byte(tc.in), tc.dst); err == nil {
			t.Errorf("Unmarshal(%s into %T) accepted out-of-range value -> %v", tc.in, tc.dst, reflect.ValueOf(tc.dst).Elem())
		}
	}
	var i8 I8
	if err := alosjson.Unmarshal([]byte(`{"N":-128}`), &i8); err != nil || i8.N != -128 {
		t.Errorf("int8 -128: %v %d", err, i8.N)
	}
	var u64 U64
	if err := alosjson.Unmarshal([]byte(`{"N":18446744073709551615}`), &u64); err != nil || u64.N != math.MaxUint64 {
		t.Errorf("uint64 max: %v %d", err, u64.N)
	}
	var i I
	if err := alosjson.Unmarshal([]byte(`{"N":3.9}`), &i); err != nil || i.N != 3 {
		t.Errorf("lenient truncation lost: %v %d", err, i.N)
	}
	if err := alosjson.Unmarshal([]byte(`{"N":"42"}`), &i); err != nil || i.N != 42 {
		t.Errorf("lenient quoted number lost: %v %d", err, i.N)
	}
}

type EmbeddedInner struct {
	A int `json:"A"`
	S string
}

type embeddedLower struct {
	L int
}

type EmbeddedOuter struct {
	EmbeddedInner
	B int
}

type EmbeddedPtrOuter struct {
	*EmbeddedInner
	B int
}

type EmbeddedNamed struct {
	EmbeddedInner `json:"inner"`
	B             int
}

type EmbeddedUnexported struct {
	embeddedLower
	B int
}

type ConflictA struct{ X, Y int }
type ConflictB struct{ X, Z int }

type EmbeddedConflict struct {
	ConflictA
	ConflictB
}

type EmbeddedShadow struct {
	EmbeddedInner
	A string
}

func TestEmbeddedStructsInline(t *testing.T) {
	cases := []struct {
		v    any
		want string
	}{
		{EmbeddedOuter{EmbeddedInner{A: 1, S: "s"}, 2}, `{"A":1,"S":"s","B":2}`},
		{EmbeddedPtrOuter{&EmbeddedInner{A: 1, S: "s"}, 2}, `{"A":1,"S":"s","B":2}`},
		{EmbeddedPtrOuter{nil, 2}, `{"B":2}`},
		{EmbeddedNamed{EmbeddedInner{A: 1}, 2}, `{"inner":{"A":1,"S":""},"B":2}`},
		{EmbeddedUnexported{embeddedLower{L: 5}, 2}, `{"L":5,"B":2}`},
		{EmbeddedConflict{ConflictA{1, 2}, ConflictB{3, 4}}, `{"Y":2,"Z":4}`},
		{EmbeddedShadow{EmbeddedInner{A: 1, S: "s"}, "top"}, `{"S":"s","A":"top"}`},
	}
	for _, tc := range cases {
		got, err := alosjson.Marshal(tc.v)
		if err != nil {
			t.Errorf("Marshal(%T): %v", tc.v, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("Marshal(%T) = %s, want %s", tc.v, got, tc.want)
		}
	}

	var o EmbeddedOuter
	if err := alosjson.Unmarshal([]byte(`{"A":1,"S":"s","B":2}`), &o); err != nil || o.A != 1 || o.S != "s" || o.B != 2 {
		t.Errorf("Unmarshal embedded: %v %+v", err, o)
	}
	var p EmbeddedPtrOuter
	if err := alosjson.Unmarshal([]byte(`{"A":1,"S":"s","B":2}`), &p); err != nil || p.EmbeddedInner == nil || p.A != 1 || p.B != 2 {
		t.Errorf("Unmarshal embedded pointer: %v %+v", err, p)
	}
	var sh EmbeddedShadow
	if err := alosjson.Unmarshal([]byte(`{"A":"top","S":"s"}`), &sh); err != nil || sh.A != "top" || sh.S != "s" || sh.EmbeddedInner.A != 0 {
		t.Errorf("Unmarshal shadowed: %v %+v", err, sh)
	}
}

func TestEmptyArrayDecodesNonNil(t *testing.T) {
	var s []int
	if err := alosjson.Unmarshal([]byte(`[]`), &s); err != nil || s == nil || len(s) != 0 {
		t.Errorf("[] into nil slice: err=%v nil=%v len=%d", err, s == nil, len(s))
	}
	type W struct{ L []string }
	var w W
	if err := alosjson.Unmarshal([]byte(`{"L":[]}`), &w); err != nil || w.L == nil {
		t.Errorf("[] into struct slice field: err=%v nil=%v", err, w.L == nil)
	}
	s = []int{1, 2}
	if err := alosjson.Unmarshal([]byte(`null`), &s); err != nil || s != nil {
		t.Errorf("null into slice: err=%v s=%v", err, s)
	}
}

func overwrite(b []byte) {
	for i := range b {
		b[i] = 'X'
	}
}

func TestDecodedStringsDoNotAliasInput(t *testing.T) {
	type S struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
		Any  any      `json:"any"`
		M    map[string]string
	}
	src := `{"name":"alice","tags":["t1","t2"],"any":{"k":"v","arr":["x"]},"M":{"key":"val"}}`
	buf := []byte(src)
	var s S
	if err := alosjson.Unmarshal(buf, &s); err != nil {
		t.Fatal(err)
	}
	var generic any
	buf2 := []byte(src)
	if err := alosjson.Unmarshal(buf2, &generic); err != nil {
		t.Fatal(err)
	}
	wantGeneric := map[string]any{"name": "alice", "tags": []any{"t1", "t2"}, "any": map[string]any{"k": "v", "arr": []any{"x"}}, "M": map[string]any{"key": "val"}}
	overwrite(buf)
	overwrite(buf2)
	if s.Name != "alice" || s.Tags[0] != "t1" || s.Tags[1] != "t2" || s.M["key"] != "val" {
		t.Errorf("struct strings changed after input overwrite: %+v", s)
	}
	if !reflect.DeepEqual(s.Any, wantGeneric["any"]) {
		t.Errorf("struct any changed after input overwrite: %v", s.Any)
	}
	if !reflect.DeepEqual(generic, wantGeneric) {
		t.Errorf("generic value changed after input overwrite: %v", generic)
	}

	first := []byte(`{"name":"first-value"}`)
	var a, b S
	if err := alosjson.Unmarshal(first, &a); err != nil {
		t.Fatal(err)
	}
	second := []byte(`{"name":"first-value"}`)
	if err := alosjson.Unmarshal(second, &b); err != nil {
		t.Fatal(err)
	}
	overwrite(first)
	if b.Name != "first-value" {
		t.Errorf("second decode aliases first input: %q", b.Name)
	}
}

func largeStreamingDoc() []byte {
	var sb strings.Builder
	sb.WriteString(`{"items":[`)
	for i := range 2000 {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`{"id":123456789,"name":"item-name-with-some-length","price":12345.6789,"ok":true,"none":null,"tags":["a","bb","ccc"]}`)
	}
	sb.WriteString(`],"count":2000}`)
	return []byte(sb.String())
}

func TestUnmarshalReadMatchesUnmarshal(t *testing.T) {
	doc := largeStreamingDoc()
	var want any
	if err := alosjson.Unmarshal(doc, &want); err != nil {
		t.Fatal(err)
	}
	readers := map[string]func() io.Reader{
		"whole":   func() io.Reader { return bytes.NewReader(doc) },
		"onebyte": func() io.Reader { return iotest.OneByteReader(bytes.NewReader(doc)) },
		"half":    func() io.Reader { return iotest.HalfReader(bytes.NewReader(doc)) },
	}
	for name, mk := range readers {
		var got any
		if err := alosjson.UnmarshalRead(mk(), &got); err != nil {
			t.Errorf("%s: UnmarshalRead: %v", name, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: UnmarshalRead result differs from Unmarshal", name)
		}
	}
}

func TestStreamingDecoderTokensAcrossReads(t *testing.T) {
	doc := `{"key":"value-that-is-long","n":1234567890,"f":-12.5e3,"t":true,"z":null,"a":[false]}`
	dec := jsontext.NewDecoder(iotest.OneByteReader(strings.NewReader(doc)))
	defer jsontext.PutDecoder(dec)
	var got []string
	for {
		tok, err := dec.ReadToken()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("ReadToken after %v: %v", got, err)
		}
		got = append(got, tok.String())
	}
	want := []string{"{", "key", "value-that-is-long", "n", "1234567890", "f", "-12.5e3", "t", "true", "z", "null", "a", "[", "false", "]", "}"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tokens = %q, want %q", got, want)
	}
}

type Level int

func (l Level) MarshalJSON() ([]byte, error) { return []byte(`"level"`), nil }

func (l *Level) UnmarshalJSON(b []byte) error {
	*l = 7
	return nil
}

type Upper string

func (u Upper) MarshalText() ([]byte, error) { return []byte(strings.ToUpper(string(u))), nil }

func TestNamedScalarCustomMarshalers(t *testing.T) {
	type S struct {
		L  Level   `json:"l"`
		U  Upper   `json:"u"`
		US []Upper `json:"us"`
	}
	got, err := alosjson.Marshal(S{L: 1, U: "ab", US: []Upper{"cd"}})
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"l":"level","u":"AB","us":["CD"]}`; string(got) != want {
		t.Errorf("Marshal = %s, want %s", got, want)
	}
	if got, _ := alosjson.Marshal(map[string]any{"l": Level(1)}); string(got) != `{"l":"level"}` {
		t.Errorf("Marshal(any(Level)) = %s", got)
	}
	var s S
	if err := alosjson.Unmarshal([]byte(`{"l":1}`), &s); err != nil || s.L != 7 {
		t.Errorf("Unmarshal named int with UnmarshalJSON: err=%v L=%d", err, s.L)
	}
}

func TestStringsWithSpacesAreNotEscaped(t *testing.T) {
	long := strings.Repeat("alpha bravo ", 100)
	got, err := alosjson.Marshal(long)
	if err != nil {
		t.Fatal(err)
	}
	if want := `"` + long + `"`; string(got) != want {
		t.Errorf("Marshal(spaces) escaped bytes: got %d bytes, want %d", len(got), len(want))
	}
}

type PointerShaped struct{ P *int }
type MapShaped struct{ M map[string]int }
type NestedPointerShaped struct{ Inner PointerShaped }

func TestPointerShapedStructsByValue(t *testing.T) {
	n := 42
	cases := []struct {
		v    any
		want string
	}{
		{PointerShaped{P: &n}, `{"P":42}`},
		{PointerShaped{}, `{"P":null}`},
		{MapShaped{M: map[string]int{"a": 1}}, `{"M":{"a":1}}`},
		{NestedPointerShaped{Inner: PointerShaped{P: &n}}, `{"Inner":{"P":42}}`},
		{[]PointerShaped{{P: &n}}, `[{"P":42}]`},
		{map[string]PointerShaped{"k": {P: &n}}, `{"k":{"P":42}}`},
		{[]any{PointerShaped{P: &n}}, `[{"P":42}]`},
	}
	for _, tc := range cases {
		got, err := alosjson.Marshal(tc.v)
		if err != nil {
			t.Errorf("Marshal(%T): %v", tc.v, err)
			continue
		}
		if string(got) != tc.want {
			t.Errorf("Marshal(%T) = %s, want %s", tc.v, got, tc.want)
		}
	}
}

func TestInvalidUTF8PassesThroughEscapedStrings(t *testing.T) {
	doc := []byte("[\"a\xffb\\n\xc3\", \"plain\xfe\"]")
	var got []string
	if err := alosjson.Unmarshal(doc, &got); err != nil {
		t.Fatal(err)
	}
	if want := []string{"a\xffb\n\xc3", "plain\xfe"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
	var ref []string
	if err := alosjson.UnmarshalReference(doc, &ref); err != nil || !reflect.DeepEqual(ref, got) {
		t.Errorf("reference got %q err %v", ref, err)
	}
}
