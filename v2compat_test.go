//go:build goexperiment.jsonv2

package json_test

import (
	jsonv2 "encoding/json/v2"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"

	json "github.com/guno1928/alos-json"
)

type stringifyComparable struct {
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
	Q   int `json:",string"`
}

type v2MarshalCase struct {
	name string
	v    any
	alos []json.Option
	std  []jsonv2.Options
}

func v2MarshalCases() []v2MarshalCase {
	celsiusAlos := json.WithMarshalers(json.MarshalFunc(formatCelsius))
	celsiusStd := jsonv2.WithMarshalers(jsonv2.MarshalFunc(formatCelsius))
	celsiusPtrAlos := json.WithMarshalers(json.MarshalFunc(func(c *celsius) ([]byte, error) { return formatCelsius(*c) }))
	celsiusPtrStd := jsonv2.WithMarshalers(jsonv2.MarshalFunc(func(c *celsius) ([]byte, error) { return formatCelsius(*c) }))
	stringerFn := func(s fmt.Stringer) ([]byte, error) { return []byte(strconv.Quote(s.String())), nil }
	firstFn := func(int) ([]byte, error) { return []byte(`"first"`), nil }
	secondFn := func(int) ([]byte, error) { return []byte(`"second"`), nil }
	methodFn := func(methodType) ([]byte, error) { return []byte(`"func"`), nil }
	report := celsiusReport{T: 21.5, L: []celsius{1}, M: map[string]celsius{"k": 2}, A: celsius(3), P: ptrTo(celsius(-4))}
	return []v2MarshalCase{
		{"stringify", stringifyComparable{I: -3, U8: 200, F: 1.5, F32: 0.25, P: ptrTo[int64](9), S: []int{1, 2}, U: []uint32{7}, M: map[string]int{"k": 4}, K: map[int]int{1: 2}, A: 2.0, B: true, Str: "x", Q: 7},
			[]json.Option{json.StringifyNumbers(true), json.Deterministic(true)}, []jsonv2.Options{jsonv2.StringifyNumbers(true), jsonv2.Deterministic(true)}},
		{"stringify simple", stringifySimple{A: 1, B: 2.5}, []json.Option{json.StringifyNumbers(true)}, []jsonv2.Options{jsonv2.StringifyNumbers(true)}},
		{"stringify any", []any{1.0, 2.5, "s", nil, true}, []json.Option{json.StringifyNumbers(true)}, []jsonv2.Options{jsonv2.StringifyNumbers(true)}},
		{"stringify top", 5, []json.Option{json.StringifyNumbers(true)}, []jsonv2.Options{jsonv2.StringifyNumbers(true)}},
		{"omitzero all", omitZeroAll{E: []int{}, F: map[string]int{}, G: negativeIsZero{N: -1}}, []json.Option{json.OmitZeroStructFields(true)}, []jsonv2.Options{jsonv2.OmitZeroStructFields(true)}},
		{"omitzero isZero false", omitZeroAll{G: negativeIsZero{N: 0}}, []json.Option{json.OmitZeroStructFields(true)}, []jsonv2.Options{jsonv2.OmitZeroStructFields(true)}},
		{"omitzero mixed", omitZeroAll{A: 1, C: ptrTo(0), D: omitZeroInner{X: 2}, J: [2]int{0, 1}}, []json.Option{json.OmitZeroStructFields(true)}, []jsonv2.Options{jsonv2.OmitZeroStructFields(true)}},
		{"omitzero simple", []omitZeroSimple{{}, {A: 3}, {B: "b"}}, []json.Option{json.OmitZeroStructFields(true)}, []jsonv2.Options{jsonv2.OmitZeroStructFields(true)}},
		{"inline struct", inlinePerson{Name: "a", Address: inlineAddress{City: "c"}}, nil, nil},
		{"inline struct ptr", inlinePerson{Name: "a", Address: inlineAddress{City: "c", Zip: "z"}, Meta: &inlineMeta{Version: 2}}, nil, nil},
		{"inline fallback any", fallbackAny{ID: 1, Extra: map[string]any{"b": 2, "a": "x"}}, []json.Option{json.Deterministic(true)}, []jsonv2.Options{jsonv2.Deterministic(true)}},
		{"inline fallback nil", fallbackAny{ID: 1}, nil, nil},
		{"inline fallback empty", fallbackAny{ID: 1, Extra: map[string]any{}}, nil, nil},
		{"inline fallback omitted field", fallbackOmit{Rest: map[string]int{"id": 5}}, nil, nil},
		{"inline fallback first", fallbackOmit{Rest: map[string]int{"b": 2, "a": 1}}, []json.Option{json.Deterministic(true)}, []jsonv2.Options{jsonv2.Deterministic(true)}},
		{"inline fallback through nil ptr", fallbackThroughPointer{Name: "x"}, nil, nil},
		{"inline fallback through ptr", fallbackThroughPointer{Name: "x", FallbackHolder: &FallbackHolder{Rest: map[string]int{"z": 1}}}, nil, nil},
		{"inline fallback duplicate", fallbackOmit{ID: 1, Rest: map[string]int{"id": 5}}, nil, nil},
		{"inline fallback duplicate any", fallbackAny{ID: 1, Extra: map[string]any{"id": 5}}, nil, nil},
		{"inline fallback case differs", fallbackAny{ID: 1, Extra: map[string]any{"ID": 5}}, nil, nil},
		{"marshal func", report, []json.Option{celsiusAlos}, []jsonv2.Options{celsiusStd}},
		{"marshal func pointer", report, []json.Option{celsiusPtrAlos}, []jsonv2.Options{celsiusPtrStd}},
		{"marshal func top", celsius(5), []json.Option{celsiusAlos}, []jsonv2.Options{celsiusStd}},
		{"marshal func top ptr", ptrTo(celsius(5)), []json.Option{celsiusAlos}, []jsonv2.Options{celsiusStd}},
		{"marshal func interface", map[string]any{"a": stringerTag("x"), "b": 1}, []json.Option{json.WithMarshalers(json.MarshalFunc(stringerFn)), json.Deterministic(true)}, []jsonv2.Options{jsonv2.WithMarshalers(jsonv2.MarshalFunc(stringerFn)), jsonv2.Deterministic(true)}},
		{"marshal func beats method", []methodType{1}, []json.Option{json.WithMarshalers(json.MarshalFunc(methodFn))}, []jsonv2.Options{jsonv2.WithMarshalers(jsonv2.MarshalFunc(methodFn))}},
		{"join order", []int{1}, []json.Option{json.WithMarshalers(json.JoinMarshalers(json.MarshalFunc(firstFn), json.MarshalFunc(secondFn)))}, []jsonv2.Options{jsonv2.WithMarshalers(jsonv2.JoinMarshalers(jsonv2.MarshalFunc(firstFn), jsonv2.MarshalFunc(secondFn)))}},
		{"marshal func compacts", []celsius{1}, []json.Option{json.WithMarshalers(json.MarshalFunc(func(celsius) ([]byte, error) { return []byte(" { \"v\" : [ 1 , 2 ] } "), nil }))}, []jsonv2.Options{jsonv2.WithMarshalers(jsonv2.MarshalFunc(func(celsius) ([]byte, error) { return []byte(" { \"v\" : [ 1 , 2 ] } "), nil }))}},
		{"marshal func invalid", celsius(1), []json.Option{json.WithMarshalers(json.MarshalFunc(func(celsius) ([]byte, error) { return []byte(`{"a":`), nil }))}, []jsonv2.Options{jsonv2.WithMarshalers(jsonv2.MarshalFunc(func(celsius) ([]byte, error) { return []byte(`{"a":`), nil }))}},
		{"marshal func map keys", map[string]int{"b": 1, "a": 2}, []json.Option{json.WithMarshalers(json.MarshalFunc(upperString)), json.Deterministic(true)}, []jsonv2.Options{jsonv2.WithMarshalers(jsonv2.MarshalFunc(upperString)), jsonv2.Deterministic(true)}},
		{"marshal func map keys sorted by output", map[string]int{"b": 1, "a": 2}, []json.Option{json.WithMarshalers(json.MarshalFunc(renameA)), json.Deterministic(true)}, []jsonv2.Options{jsonv2.WithMarshalers(jsonv2.MarshalFunc(renameA)), jsonv2.Deterministic(true)}},
		{"marshal func fallback keys", fallbackAny{ID: 1, Extra: map[string]any{"a": "b"}}, []json.Option{json.WithMarshalers(json.MarshalFunc(upperString))}, []jsonv2.Options{jsonv2.WithMarshalers(jsonv2.MarshalFunc(upperString))}},
		{"marshal func non-string key", map[int]string{1: "x"}, []json.Option{json.WithMarshalers(json.MarshalFunc(func(n int) ([]byte, error) { return []byte(strconv.Itoa(n)), nil }))}, []jsonv2.Options{jsonv2.WithMarshalers(jsonv2.MarshalFunc(func(n int) ([]byte, error) { return []byte(strconv.Itoa(n)), nil }))}},
		{"marshal func skip", celsius(1), []json.Option{json.WithMarshalers(json.MarshalFunc(func(celsius) ([]byte, error) { return nil, json.SkipFunc }))}, []jsonv2.Options{jsonv2.WithMarshalers(jsonv2.MarshalFunc(func(celsius) ([]byte, error) { return nil, jsonv2.SkipFunc }))}},
	}
}

func TestMatchesEncodingJSONV2Marshal(t *testing.T) {
	for _, c := range v2MarshalCases() {
		t.Run(c.name, func(t *testing.T) {
			got, errAlos := json.Marshal(c.v, c.alos...)
			want, errStd := jsonv2.Marshal(c.v, c.std...)
			if (errAlos != nil) != (errStd != nil) {
				t.Fatalf("error mismatch: alos %v, v2 %v (alos %s, v2 %s)", errAlos, errStd, got, want)
			}
			if errStd == nil && string(got) != string(want) {
				t.Fatalf("output mismatch\nalos %s\n  v2 %s", got, want)
			}
		})
	}
}

func compareV2Decode[T any](t *testing.T, data string, alosOpts []json.Option, stdOpts []jsonv2.Options) {
	t.Helper()
	var got, want T
	errAlos := json.Unmarshal([]byte(data), &got, alosOpts...)
	errStd := jsonv2.Unmarshal([]byte(data), &want, stdOpts...)
	if (errAlos != nil) != (errStd != nil) {
		t.Fatalf("error mismatch for %s: alos %v, v2 %v", data, errAlos, errStd)
	}
	if errStd == nil && !reflect.DeepEqual(got, want) {
		t.Fatalf("value mismatch for %s\nalos %#v\n  v2 %#v", data, got, want)
	}
}

func TestMatchesEncodingJSONV2Unmarshal(t *testing.T) {
	celsiusAlos := []json.Option{json.WithUnmarshalers(json.UnmarshalFunc(parseCelsius))}
	celsiusStd := []jsonv2.Options{jsonv2.WithUnmarshalers(jsonv2.UnmarshalFunc(parseCelsius))}
	upperFn := func(b []byte, s *string) error {
		v, err := strconv.Unquote(string(b))
		*s = strings.ToUpper(v)
		return err
	}
	upperAlos := []json.Option{json.WithUnmarshalers(json.UnmarshalFunc(upperFn))}
	upperStd := []jsonv2.Options{jsonv2.WithUnmarshalers(jsonv2.UnmarshalFunc(upperFn))}
	strictAlos := []json.Option{json.RejectUnknownMembers(true)}
	strictStd := []jsonv2.Options{jsonv2.RejectUnknownMembers(true)}

	t.Run("inline struct", func(t *testing.T) {
		compareV2Decode[inlinePerson](t, `{"name":"a","city":"c","zip":"z","version":3}`, nil, nil)
		compareV2Decode[inlinePerson](t, `{"name":"b"}`, nil, nil)
	})
	t.Run("inline fallback", func(t *testing.T) {
		compareV2Decode[fallbackAny](t, `{"id":1,"a":"x","b":[1,2],"c":null}`, nil, nil)
		compareV2Decode[fallbackAny](t, `{"id":2,"z":true}`, strictAlos, strictStd)
		compareV2Decode[fallbackOmit](t, `{"id":1,"n":2}`, nil, nil)
		compareV2Decode[fallbackOmit](t, `{"id":1,"n":"x"}`, nil, nil)
		compareV2Decode[fallbackThroughPointer](t, `{"name":"x","z":1}`, nil, nil)
	})
	t.Run("unmarshal func", func(t *testing.T) {
		compareV2Decode[celsiusReport](t, `{"T":"21.5C","L":["1C","2C"],"M":{"k":"3C"},"P":"4C"}`, celsiusAlos, celsiusStd)
		compareV2Decode[celsiusReport](t, `{"T":null,"P":null}`, celsiusAlos, celsiusStd)
		compareV2Decode[celsius](t, `"7C"`, celsiusAlos, celsiusStd)
	})
	t.Run("unmarshal func any tree", func(t *testing.T) {
		compareV2Decode[map[string]any](t, `{"a":"x","b":["y",1,true,null],"c":{"d":"z"}}`, upperAlos, upperStd)
		compareV2Decode[any](t, `["q"]`, upperAlos, upperStd)
		compareV2Decode[map[int]int](t, `{"1":3,"5":-1}`, []json.Option{json.WithUnmarshalers(json.UnmarshalFunc(doubleInt))}, []jsonv2.Options{jsonv2.WithUnmarshalers(jsonv2.UnmarshalFunc(doubleInt))})
	})
}
