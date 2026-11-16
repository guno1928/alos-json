package json_test

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"testing"

	json "github.com/guno1928/alos-json"
	"github.com/guno1928/alos-json/jsontext"
)

func mustMarshal(t *testing.T, v any, opts ...json.Option) string {
	t.Helper()
	b, err := json.Marshal(v, opts...)
	if err != nil {
		t.Fatalf("Marshal(%#v): %v", v, err)
	}
	return string(b)
}

func expectMarshal(t *testing.T, v any, want string, opts ...json.Option) {
	t.Helper()
	if got := mustMarshal(t, v, opts...); got != want {
		t.Fatalf("Marshal(%#v)\n got %s\nwant %s", v, got, want)
	}
}

func expectMarshalError(t *testing.T, v any, substr string, opts ...json.Option) error {
	t.Helper()
	b, err := json.Marshal(v, opts...)
	if err == nil {
		t.Fatalf("Marshal(%#v) = %s, want error containing %q", v, b, substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("Marshal(%#v) error %q, want it to contain %q", v, err, substr)
	}
	return err
}

func decodeBothWays[T any](t *testing.T, data string, opts ...json.Option) T {
	t.Helper()
	var fast, stream T
	if err := json.Unmarshal([]byte(data), &fast, opts...); err != nil {
		t.Fatalf("Unmarshal(%s): %v", data, err)
	}
	if err := json.UnmarshalRead(strings.NewReader(data), &stream, opts...); err != nil {
		t.Fatalf("UnmarshalRead(%s): %v", data, err)
	}
	if !reflect.DeepEqual(fast, stream) {
		t.Fatalf("Unmarshal %#v != UnmarshalRead %#v for %s", fast, stream, data)
	}
	return fast
}

func expectDecodeErrorBothWays[T any](t *testing.T, data string, substr string, opts ...json.Option) {
	t.Helper()
	var fast, stream T
	errFast := json.Unmarshal([]byte(data), &fast, opts...)
	errStream := json.UnmarshalRead(strings.NewReader(data), &stream, opts...)
	for name, err := range map[string]error{"Unmarshal": errFast, "UnmarshalRead": errStream} {
		if err == nil || !strings.Contains(err.Error(), substr) {
			t.Fatalf("%s(%s) error = %v, want it to contain %q", name, data, err, substr)
		}
	}
}

func ptrTo[T any](v T) *T { return &v }

type stringifyAll struct {
	I   int
	U8  uint8
	F   float64
	F32 float32
	P   *int64
	S   []int
	U   []uint32
	M   map[string]int
	K   map[int]int
	A   any
	B   bool
	Str string
	Q   int  `json:",string"`
	QB  bool `json:",string"`
}

type stringifySimple struct {
	A int
	B float64
}

func TestStringifyNumbersMarshal(t *testing.T) {
	on := json.StringifyNumbers(true)
	v := stringifyAll{
		I: -3, U8: 200, F: 1.5, F32: 0.25, P: ptrTo[int64](9),
		S: []int{1, 2}, U: []uint32{7}, M: map[string]int{"k": 4}, K: map[int]int{1: 2},
		A: 2.0, B: true, Str: "x", Q: 7, QB: true,
	}
	expectMarshal(t, v, `{"I":"-3","U8":"200","F":"1.5","F32":"0.25","P":"9","S":["1","2"],"U":["7"],"M":{"k":"4"},"K":{"1":"2"},"A":"2","B":true,"Str":"x","Q":"7","QB":"true"}`, on)
	expectMarshal(t, v, `{"I":-3,"U8":200,"F":1.5,"F32":0.25,"P":9,"S":[1,2],"U":[7],"M":{"k":4},"K":{"1":2},"A":2,"B":true,"Str":"x","Q":"7","QB":"true"}`)
	expectMarshal(t, stringifySimple{A: 1, B: 2.5}, `{"A":"1","B":"2.5"}`, on)
	expectMarshal(t, 5, `"5"`, on)
	expectMarshal(t, []any{1, int64(2), 3.5, "s", nil}, `["1","2","3.5","s",null]`, on)
	expectMarshal(t, map[string]any{"n": 1e21}, `{"n":"1e+21"}`, on)
	expectMarshalError(t, stringifySimple{B: nan()}, "unsupported float", on)
}

func nan() float64 {
	zero := 0.0
	return zero / zero
}

func TestStringifyNumbersRoundTrip(t *testing.T) {
	data := mustMarshal(t, stringifySimple{A: -12, B: 0.125}, json.StringifyNumbers(true))
	got := decodeBothWays[stringifySimple](t, data, json.StringifyNumbers(true))
	if got != (stringifySimple{A: -12, B: 0.125}) {
		t.Fatalf("round trip = %+v", got)
	}
}

type negativeIsZero struct{ N int }

func (z negativeIsZero) IsZero() bool { return z.N < 0 }

type omitZeroInner struct{ X int }

type omitZeroAll struct {
	A int
	B string
	C *int
	D omitZeroInner
	E []int
	F map[string]int
	G negativeIsZero
	H bool
	I any
	J [2]int
}

type omitZeroSimple struct {
	A int
	B string
}

func TestOmitZeroStructFields(t *testing.T) {
	on := json.OmitZeroStructFields(true)
	expectMarshal(t, omitZeroAll{E: []int{}, F: map[string]int{}, G: negativeIsZero{N: -1}}, `{"E":[],"F":{}}`, on)
	expectMarshal(t, omitZeroAll{G: negativeIsZero{N: 0}}, `{"G":{}}`, on)
	expectMarshal(t, omitZeroAll{G: negativeIsZero{N: 3}}, `{"G":{"N":3}}`, on)
	expectMarshal(t, omitZeroAll{A: 1, C: ptrTo(0), D: omitZeroInner{X: 2}, J: [2]int{0, 1}}, `{"A":1,"C":0,"D":{"X":2},"G":{},"J":[0,1]}`, on)
	expectMarshal(t, omitZeroSimple{}, `{}`, on)
	expectMarshal(t, omitZeroSimple{B: "b"}, `{"B":"b"}`, on)
	expectMarshal(t, omitZeroSimple{}, `{"A":0,"B":""}`)
	expectMarshal(t, omitZeroSimple{}, `{"A":0,"B":""}`, json.OmitZeroStructFields(false))
	expectMarshal(t, []omitZeroSimple{{}, {A: 3}}, `[{},{"A":3}]`, on)
}

type nocaseFields struct {
	Lower string `json:"value,nocase"`
	Upper string `json:"VALUE"`
	Exact string `json:"exact"`
	Name  string `json:",nocase"`
}

func TestNocaseTag(t *testing.T) {
	got := decodeBothWays[nocaseFields](t, `{"VALUE":"u","Value":"x","exact":"e","NAME":"n"}`)
	if want := (nocaseFields{Lower: "x", Upper: "u", Exact: "e", Name: "n"}); got != want {
		t.Fatalf("nocase = %+v, want %+v", got, want)
	}
	got = decodeBothWays[nocaseFields](t, `{"value":"l","EXACT":"ignored","name":"m"}`)
	if want := (nocaseFields{Lower: "l", Name: "m"}); got != want {
		t.Fatalf("nocase exact = %+v, want %+v", got, want)
	}
	got = decodeBothWays[nocaseFields](t, `{"EXACT":"e"}`, json.MatchCaseInsensitiveNames(true))
	if got.Exact != "e" {
		t.Fatalf("MatchCaseInsensitiveNames with nocase = %+v", got)
	}
	expectDecodeErrorBothWays[nocaseFields](t, `{"EXACT":"e"}`, "unknown field", json.RejectUnknownMembers(true))
	expectMarshal(t, nocaseFields{Lower: "a", Name: "n"}, `{"value":"a","VALUE":"","exact":"","Name":"n"}`)
}

type inlineAddress struct {
	City string `json:"city"`
	Zip  string `json:"zip,omitempty"`
}

type inlineMeta struct {
	Version int `json:"version"`
}

type inlinePerson struct {
	Name    string        `json:"name"`
	Address inlineAddress `json:",inline"`
	Meta    *inlineMeta   `json:",inline"`
}

func TestInlineNamedStruct(t *testing.T) {
	expectMarshal(t, inlinePerson{Name: "a", Address: inlineAddress{City: "c"}}, `{"name":"a","city":"c"}`)
	expectMarshal(t, inlinePerson{Name: "a", Address: inlineAddress{City: "c", Zip: "z"}, Meta: &inlineMeta{Version: 2}}, `{"name":"a","city":"c","zip":"z","version":2}`)
	got := decodeBothWays[inlinePerson](t, `{"name":"a","city":"c","zip":"z","version":3}`)
	if got.Name != "a" || got.Address != (inlineAddress{City: "c", Zip: "z"}) || got.Meta == nil || got.Meta.Version != 3 {
		t.Fatalf("inline decode = %+v (meta %+v)", got, got.Meta)
	}
	got = decodeBothWays[inlinePerson](t, `{"name":"b"}`)
	if got.Meta != nil {
		t.Fatalf("inline pointer allocated without members: %+v", got.Meta)
	}
}

type fallbackAny struct {
	ID    int            `json:"id"`
	Extra map[string]any `json:",inline"`
}

type fallbackOmit struct {
	ID   int            `json:"id,omitzero"`
	Rest map[string]int `json:",inline"`
}

type Labels map[string]string

type fallbackEmbedded struct {
	Name   string `json:"name"`
	Labels `json:",inline"`
}

type FallbackHolder struct {
	Rest map[string]int `json:",inline"`
}

type fallbackThroughPointer struct {
	Name string `json:"name"`
	*FallbackHolder
}

type fallbackNocase struct {
	Key  string         `json:"key,nocase"`
	Rest map[string]int `json:",inline"`
}

func TestInlineFallbackDecode(t *testing.T) {
	got := decodeBothWays[fallbackAny](t, `{"id":1,"a":"x","b":[1,2],"c":null}`)
	want := fallbackAny{ID: 1, Extra: map[string]any{"a": "x", "b": []any{1.0, 2.0}, "c": nil}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("fallback = %#v, want %#v", got, want)
	}
	strict := decodeBothWays[fallbackAny](t, `{"id":2,"z":true}`, json.RejectUnknownMembers(true))
	if !reflect.DeepEqual(strict, fallbackAny{ID: 2, Extra: map[string]any{"z": true}}) {
		t.Fatalf("fallback with RejectUnknownMembers = %#v", strict)
	}
	typed := decodeBothWays[fallbackOmit](t, `{"id":1,"n":2}`)
	if !reflect.DeepEqual(typed, fallbackOmit{ID: 1, Rest: map[string]int{"n": 2}}) {
		t.Fatalf("typed fallback = %#v", typed)
	}
	expectDecodeErrorBothWays[fallbackOmit](t, `{"id":1,"n":"x"}`, "")
	embedded := decodeBothWays[fallbackEmbedded](t, `{"name":"n","k":"v"}`)
	if !reflect.DeepEqual(embedded, fallbackEmbedded{Name: "n", Labels: Labels{"k": "v"}}) {
		t.Fatalf("embedded fallback = %#v", embedded)
	}
	through := decodeBothWays[fallbackThroughPointer](t, `{"name":"x","z":1}`)
	if through.FallbackHolder == nil || !reflect.DeepEqual(through.Rest, map[string]int{"z": 1}) {
		t.Fatalf("fallback through pointer = %#v", through)
	}
	keyed := decodeBothWays[fallbackNocase](t, `{"KEY":"k","other":3}`)
	if !reflect.DeepEqual(keyed, fallbackNocase{Key: "k", Rest: map[string]int{"other": 3}}) {
		t.Fatalf("nocase with fallback = %#v", keyed)
	}
	escaped := decodeBothWays[fallbackOmit](t, `{"a\"b":1,"id":5}`)
	if !reflect.DeepEqual(escaped, fallbackOmit{ID: 5, Rest: map[string]int{`a"b`: 1}}) {
		t.Fatalf("escaped fallback key = %#v", escaped)
	}
}

func TestInlineFallbackMarshal(t *testing.T) {
	det := json.Deterministic(true)
	expectMarshal(t, fallbackAny{ID: 1, Extra: map[string]any{"b": 2, "a": "x"}}, `{"id":1,"a":"x","b":2}`, det)
	expectMarshal(t, fallbackAny{ID: 1}, `{"id":1}`)
	expectMarshal(t, fallbackAny{ID: 1, Extra: map[string]any{}}, `{"id":1}`)
	expectMarshal(t, fallbackOmit{Rest: map[string]int{"id": 5}}, `{"id":5}`)
	expectMarshal(t, fallbackOmit{Rest: map[string]int{"b": 2, "a": 1}}, `{"a":1,"b":2}`, det)
	expectMarshal(t, fallbackEmbedded{Name: "n", Labels: Labels{"k": "v"}}, `{"name":"n","k":"v"}`)
	expectMarshal(t, fallbackThroughPointer{Name: "x"}, `{"name":"x"}`)
	expectMarshal(t, fallbackThroughPointer{Name: "x", FallbackHolder: &FallbackHolder{Rest: map[string]int{"z": 1}}}, `{"name":"x","z":1}`)
	expectMarshalError(t, fallbackOmit{ID: 1, Rest: map[string]int{"id": 5}}, "duplicate object member name")
	expectMarshalError(t, fallbackAny{ID: 1, Extra: map[string]any{"id": 5}}, "duplicate object member name")
	expectMarshalError(t, fallbackNocase{Key: "k", Rest: map[string]int{"KEY": 1}}, "duplicate object member name")
	expectMarshal(t, fallbackAny{ID: 1, Extra: map[string]any{"ID": 5}}, `{"id":1,"ID":5}`)
	expectMarshalError(t, fallbackAny{ID: 1, Extra: map[string]any{"ID": 5}}, "duplicate object member name", json.MatchCaseInsensitiveNames(true))
	indented := mustMarshal(t, fallbackAny{ID: 1, Extra: map[string]any{"a": []int{1}}}, json.WithIndent("", "  "))
	if want := "{\n  \"id\": 1,\n  \"a\": [\n    1\n  ]\n}"; indented != want {
		t.Fatalf("indented fallback = %q, want %q", indented, want)
	}
}

type celsius float64

type celsiusReport struct {
	T celsius
	L []celsius
	M map[string]celsius
	A any
	P *celsius
}

type methodType int

func (methodType) MarshalJSON() ([]byte, error) { return []byte(`"method"`), nil }

type stringerTag string

func (s stringerTag) String() string { return "tag:" + string(s) }

func formatCelsius(c celsius) ([]byte, error) {
	return []byte(strconv.Quote(strconv.FormatFloat(float64(c), 'g', -1, 64) + "C")), nil
}

func TestMarshalFunc(t *testing.T) {
	byValue := json.WithMarshalers(json.MarshalFunc(formatCelsius))
	report := celsiusReport{T: 21.5, L: []celsius{1}, M: map[string]celsius{"k": 2}, A: celsius(3)}
	expectMarshal(t, report, `{"T":"21.5C","L":["1C"],"M":{"k":"2C"},"A":"3C","P":null}`, byValue)
	expectMarshal(t, report, `{"T":21.5,"L":[1],"M":{"k":2},"A":3,"P":null}`)

	byPointer := json.WithMarshalers(json.MarshalFunc(func(c *celsius) ([]byte, error) { return formatCelsius(*c) }))
	report.P = ptrTo(celsius(-4))
	expectMarshal(t, report, `{"T":"21.5C","L":["1C"],"M":{"k":"2C"},"A":"3C","P":"-4C"}`, byPointer)
	expectMarshal(t, celsius(5), `"5C"`, byValue)
	expectMarshal(t, ptrTo(celsius(5)), `"5C"`, byValue)
	expectMarshal(t, celsius(5), `5`)

	stringers := json.WithMarshalers(json.MarshalFunc(func(s fmt.Stringer) ([]byte, error) { return []byte(strconv.Quote(s.String())), nil }))
	expectMarshal(t, map[string]any{"a": stringerTag("x"), "b": 1}, `{"a":"tag:x","b":1}`, stringers, json.Deterministic(true))

	overrides := json.WithMarshalers(json.MarshalFunc(func(methodType) ([]byte, error) { return []byte(`"func"`), nil }))
	expectMarshal(t, []methodType{1}, `["func"]`, overrides)
	expectMarshal(t, []methodType{1}, `["method"]`)

	first := json.MarshalFunc(func(int) ([]byte, error) { return []byte(`"first"`), nil })
	second := json.MarshalFunc(func(int) ([]byte, error) { return []byte(`"second"`), nil })
	expectMarshal(t, []int{1}, `["first"]`, json.WithMarshalers(json.JoinMarshalers(first, second)))
	expectMarshal(t, []int{1}, `["second"]`, json.WithMarshalers(json.JoinMarshalers(nil, second, first)))

	spaced := json.WithMarshalers(json.MarshalFunc(func(celsius) ([]byte, error) { return []byte(" { \"v\" : [ 1 , 2 ] } "), nil }))
	expectMarshal(t, []celsius{1}, `[{"v":[1,2]}]`, spaced)
	indented := mustMarshal(t, []celsius{1}, spaced, json.WithIndent("", " "))
	if want := "[\n {\n  \"v\": [\n   1,\n   2\n  ]\n }\n]"; indented != want {
		t.Fatalf("indented custom output = %q, want %q", indented, want)
	}

	appended, err := json.MarshalAppend([]byte("x="), celsius(1), byValue)
	if err != nil || string(appended) != `x="1C"` {
		t.Fatalf("MarshalAppend = %s, %v", appended, err)
	}
}

var errBoom = errors.New("boom")

func TestMarshalFuncErrors(t *testing.T) {
	failing := json.WithMarshalers(json.MarshalFunc(func(celsius) ([]byte, error) { return nil, errBoom }))
	err := expectMarshalError(t, celsiusReport{}, "boom", failing)
	var se *json.SemanticError
	if !errors.Is(err, errBoom) || !errors.As(err, &se) {
		t.Fatalf("error %v does not wrap errBoom in a SemanticError", err)
	}
	expectMarshalError(t, celsius(1), "cannot be skipped", json.WithMarshalers(json.MarshalFunc(func(celsius) ([]byte, error) { return nil, json.SkipFunc })))
	expectMarshalError(t, celsius(1), "", json.WithMarshalers(json.MarshalFunc(func(celsius) ([]byte, error) { return []byte(`{"a":`), nil })))
	expectMarshalError(t, celsius(1), "", json.WithMarshalers(json.MarshalFunc(func(celsius) ([]byte, error) { return []byte(`1 2`), nil })))
}

func TestMarshalToFunc(t *testing.T) {
	positives := json.WithMarshalers(json.MarshalToFunc(func(enc *jsontext.Encoder, v int) error {
		if v < 0 {
			return json.SkipFunc
		}
		return enc.WriteString("pos")
	}))
	expectMarshal(t, []int{1, -2}, `["pos",-2]`, positives)
	expectMarshal(t, map[string]int{"a": -1}, `{"a":-1}`, positives)

	chained := json.WithMarshalers(json.JoinMarshalers(
		json.MarshalToFunc(func(*jsontext.Encoder, int) error { return json.SkipFunc }),
		json.MarshalFunc(func(int) ([]byte, error) { return []byte(`"second"`), nil }),
	))
	expectMarshal(t, []int{1}, `["second"]`, chained)

	object := json.WithMarshalers(json.MarshalToFunc(func(enc *jsontext.Encoder, c celsius) error {
		if err := enc.WriteToken(jsontext.TBeginObject); err != nil {
			return err
		}
		if err := enc.WriteString("deg"); err != nil {
			return err
		}
		if err := enc.WriteFloat(float64(c)); err != nil {
			return err
		}
		return enc.WriteToken(jsontext.TEndObject)
	}))
	expectMarshal(t, []celsius{1.5, 2}, `[{"deg":1.5},{"deg":2}]`, object)

	expectMarshalError(t, 1, "must not read or write", json.WithMarshalers(json.MarshalToFunc(func(enc *jsontext.Encoder, v int) error {
		if err := enc.WriteString("x"); err != nil {
			return err
		}
		return json.SkipFunc
	})))
	expectMarshalError(t, 1, "exactly one value", json.WithMarshalers(json.MarshalToFunc(func(*jsontext.Encoder, int) error { return nil })))
	expectMarshalError(t, 1, "exactly one value", json.WithMarshalers(json.MarshalToFunc(func(enc *jsontext.Encoder, v int) error {
		return enc.WriteToken(jsontext.TBeginArray)
	})))
}

func parseCelsius(b []byte, c *celsius) error {
	if string(b) == "null" {
		*c = -1
		return nil
	}
	s, err := strconv.Unquote(string(b))
	if err != nil {
		return err
	}
	f, err := strconv.ParseFloat(strings.TrimSuffix(s, "C"), 64)
	*c = celsius(f)
	return err
}

func TestUnmarshalFunc(t *testing.T) {
	custom := json.WithUnmarshalers(json.UnmarshalFunc(parseCelsius))
	got := decodeBothWays[celsiusReport](t, `{"T":"21.5C","L":["1C","2C"],"M":{"k":"3C"},"P":"4C"}`, custom)
	if got.T != 21.5 || !reflect.DeepEqual(got.L, []celsius{1, 2}) || !reflect.DeepEqual(got.M, map[string]celsius{"k": 3}) || got.P == nil || *got.P != 4 {
		t.Fatalf("UnmarshalFunc = %+v", got)
	}
	nulls := decodeBothWays[celsiusReport](t, `{"T":null,"P":null}`, custom)
	if nulls.T != -1 || nulls.P != nil {
		t.Fatalf("UnmarshalFunc null handling = %+v", nulls)
	}
	top := decodeBothWays[celsius](t, `"7C"`, custom)
	if top != 7 {
		t.Fatalf("top-level UnmarshalFunc = %v", top)
	}
	plain := decodeBothWays[celsiusReport](t, `{"T":1.5}`)
	if plain.T != 1.5 {
		t.Fatalf("default decoding changed after custom use: %+v", plain)
	}

	upper := json.WithUnmarshalers(json.UnmarshalFunc(func(b []byte, s *string) error {
		v, err := strconv.Unquote(string(b))
		*s = strings.ToUpper(v)
		return err
	}))
	tree := decodeBothWays[map[string]any](t, `{"a":"x","b":["y",1,true,null],"c":{"d":"z"}}`, upper)
	wantTree := map[string]any{"A": "X", "B": []any{"Y", 1.0, true, nil}, "C": map[string]any{"D": "Z"}}
	if !reflect.DeepEqual(tree, wantTree) {
		t.Fatalf("custom string func inside any tree = %#v", tree)
	}
	dynamic := decodeBothWays[any](t, `["q"]`, upper)
	if !reflect.DeepEqual(dynamic, []any{"Q"}) {
		t.Fatalf("custom string func inside any = %#v", dynamic)
	}
	type named struct{ S string }
	if got := decodeBothWays[named](t, `{"S":"lower"}`, upper); got.S != "LOWER" {
		t.Fatalf("custom string func on field = %+v", got)
	}
	doubled := json.WithUnmarshalers(json.UnmarshalFunc(doubleInt))
	if got := decodeBothWays[map[int]int](t, `{"1":3,"5":-1}`, doubled); !reflect.DeepEqual(got, map[int]int{2: 6, 10: -2}) {
		t.Fatalf("custom int func on map keys = %v", got)
	}
}

func doubleInt(b []byte, p *int) error {
	n, err := strconv.Atoi(strings.Trim(string(b), `"`))
	*p = n * 2
	return err
}

func upperString(s string) ([]byte, error) { return []byte(strconv.Quote(strings.ToUpper(s))), nil }

func renameA(s string) ([]byte, error) {
	if s == "a" {
		s = "z"
	}
	return []byte(strconv.Quote(s)), nil
}

func TestMarshalFuncMapKeys(t *testing.T) {
	det := json.Deterministic(true)
	expectMarshal(t, map[string]int{"b": 1, "a": 2}, `{"A":2,"B":1}`, json.WithMarshalers(json.MarshalFunc(upperString)), det)
	expectMarshal(t, map[string]int{"b": 1, "a": 2}, `{"b":1,"z":2}`, json.WithMarshalers(json.MarshalFunc(renameA)), det)
	expectMarshal(t, map[string]int{"a": 2}, `{"z":2}`, json.WithMarshalers(json.MarshalFunc(renameA)))
	bareInt := json.WithMarshalers(json.MarshalFunc(func(n int) ([]byte, error) { return []byte(strconv.Itoa(n)), nil }))
	expectMarshalError(t, map[int]string{1: "x"}, "map key must encode as a JSON string", bareInt)
	expectMarshal(t, map[string]int{"k": 1}, `{"k":1}`, bareInt)
	expectMarshal(t, fallbackAny{ID: 1, Extra: map[string]any{"a": "b"}}, `{"id":1,"a":"B"}`, json.WithMarshalers(json.MarshalFunc(upperString)))
}

func TestUnmarshalFuncErrors(t *testing.T) {
	failing := json.WithUnmarshalers(json.UnmarshalFunc(func([]byte, *celsius) error { return errBoom }))
	var r celsiusReport
	err := json.Unmarshal([]byte(`{"T":1}`), &r, failing)
	var se *json.SemanticError
	if !errors.Is(err, errBoom) || !errors.As(err, &se) {
		t.Fatalf("Unmarshal error %v does not wrap errBoom in a SemanticError", err)
	}
	expectDecodeErrorBothWays[celsiusReport](t, `{"T":1}`, "cannot be skipped", json.WithUnmarshalers(json.UnmarshalFunc(func([]byte, *celsius) error { return json.SkipFunc })))
	expectDecodeErrorBothWays[celsiusReport](t, `{"T":[1,}`, "", json.WithUnmarshalers(json.UnmarshalFunc(parseCelsius)))
}

func TestUnmarshalFromFunc(t *testing.T) {
	tens := json.WithUnmarshalers(json.UnmarshalFromFunc(func(dec *jsontext.Decoder, p *int) error {
		if dec.PeekKind() != jsontext.String {
			return json.SkipFunc
		}
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		n, err := strconv.Atoi(tok.Str())
		*p = n * 10
		return err
	}))
	got := decodeBothWays[[]int](t, `[1, "2", 3]`, tens)
	if !reflect.DeepEqual(got, []int{1, 20, 3}) {
		t.Fatalf("UnmarshalFromFunc = %v", got)
	}
	type pair struct{ A, B int }
	if p := decodeBothWays[pair](t, `{"A":"4","B":5}`, tens); p != (pair{A: 40, B: 5}) {
		t.Fatalf("UnmarshalFromFunc on fields = %+v", p)
	}

	expectDecodeErrorBothWays[[]int](t, `["1"]`, "must not read or write", json.WithUnmarshalers(json.UnmarshalFromFunc(func(dec *jsontext.Decoder, p *int) error {
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		return json.SkipFunc
	})))
	expectDecodeErrorBothWays[[]int](t, `[1]`, "exactly one value", json.WithUnmarshalers(json.UnmarshalFromFunc(func(*jsontext.Decoder, *int) error { return nil })))
	expectDecodeErrorBothWays[[][]int](t, `[[1]]`, "exactly one value", json.WithUnmarshalers(json.UnmarshalFromFunc(func(dec *jsontext.Decoder, p *[]int) error {
		_, err := dec.ReadToken()
		return err
	})))
}

func TestFuncTypeValidation(t *testing.T) {
	type namedPointer *int
	cases := map[string]func(){
		"unmarshal non-pointer": func() { json.UnmarshalFunc(func([]byte, int) error { return nil }) },
		"unmarshal named ptr":   func() { json.UnmarshalFunc(func([]byte, namedPointer) error { return nil }) },
		"marshal named ptr":     func() { json.MarshalFunc(func(namedPointer) ([]byte, error) { return nil, nil }) },
	}
	for name, fn := range cases {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("expected panic")
				}
			}()
			fn()
		})
	}
}

func TestCustomFuncsConcurrent(t *testing.T) {
	marshalers := json.WithMarshalers(json.MarshalFunc(formatCelsius))
	unmarshalers := json.WithUnmarshalers(json.UnmarshalFunc(parseCelsius))
	const workers = 8
	const rounds = 200
	var wg sync.WaitGroup
	errs := make(chan error, workers)
	for w := range workers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range rounds {
				report := celsiusReport{T: celsius(w), L: []celsius{celsius(i)}, M: map[string]celsius{"k": 1}, A: celsius(2)}
				b, err := json.Marshal(report, marshalers)
				if err != nil {
					errs <- err
					return
				}
				var back celsiusReport
				if err := json.Unmarshal(b, &back, unmarshalers); err != nil {
					errs <- err
					return
				}
				if back.T != report.T || back.L[0] != report.L[0] || back.M["k"] != 1 {
					errs <- fmt.Errorf("round trip mismatch: %s -> %+v", b, back)
					return
				}
				if plain := mustMarshalNoT(report); !strings.HasPrefix(plain, `{"T":`+strconv.Itoa(w)+`,`) {
					errs <- fmt.Errorf("default encoder affected: %s", plain)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}

func mustMarshalNoT(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return "error: " + err.Error()
	}
	return string(b)
}

func FuzzInlineFallbackPaths(f *testing.F) {
	for _, seed := range []string{`{"id":1,"a":"x"}`, `{"a":{"b":[1,2]},"id":3}`, `{"ID":1,"id":2,"x":null}`, `{}`, `{"a\"":1}`, `{"id":1,"id":2}`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		var fast, stream fallbackAny
		errFast := json.Unmarshal(data, &fast)
		errStream := json.UnmarshalRead(bytes.NewReader(data), &stream)
		if (errFast == nil) != (errStream == nil) {
			t.Fatalf("error mismatch for %q: Unmarshal %v, UnmarshalRead %v", data, errFast, errStream)
		}
		if errFast != nil {
			return
		}
		if !reflect.DeepEqual(fast, stream) {
			t.Fatalf("value mismatch for %q: %#v vs %#v", data, fast, stream)
		}
		out, err := json.Marshal(fast, json.Deterministic(true))
		if err != nil {
			t.Fatalf("Marshal(%#v): %v", fast, err)
		}
		var back fallbackAny
		if err := json.Unmarshal(out, &back); err != nil || !reflect.DeepEqual(back, fast) {
			t.Fatalf("round trip of %q via %s = %#v (%v)", data, out, back, err)
		}
	})
}
