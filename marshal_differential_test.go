package json_test

import (
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	alosjson "github.com/guno1928/alos-json"
	"github.com/guno1928/alos-json/jsontext"
)

var marshalOptionSets = [][]alosjson.Option{
	{alosjson.Deterministic(true)},
	{alosjson.Deterministic(true), alosjson.EscapeHTML(true)},
	{alosjson.Deterministic(true), alosjson.FormatNilSliceAsNull(true), alosjson.FormatNilMapAsNull(true)},
}

type omitTarget struct {
	A  int            `json:"a,omitempty"`
	B  string         `json:"b,omitzero"`
	C  *int           `json:"c,omitempty"`
	D  []int          `json:"d,omitempty"`
	E  map[string]int `json:"e,omitzero"`
	F  float64        `json:"f,string"`
	G  *int           `json:"g,string"`
	H  bool           `json:"h,string"`
	I  string         `json:"i,string"`
	L  Level          `json:"l,string"`
	T  time.Time      `json:"t,omitzero"`
	TP *time.Time     `json:"tp"`
	S  fmt.Stringer   `json:"s"`
	R  jsontext.Value `json:"r,omitempty"`
}

type stringerImpl struct{ N int }

func (s stringerImpl) String() string { return "stringer" }

type textKey struct{ K string }

func (k textKey) MarshalText() ([]byte, error) { return []byte("tk-" + k.K), nil }

type valueMarshaler struct{ X int }

func (v valueMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"vm":%d}`, v.X)), nil
}

type ptrMarshaler struct{ X int }

func (p *ptrMarshaler) MarshalJSON() ([]byte, error) {
	return []byte(fmt.Sprintf(`{"pm":%d}`, p.X)), nil
}

type marshalerToImpl struct{ X int }

func (m marshalerToImpl) MarshalJSONTo(enc *jsontext.Encoder) error {
	enc.WriteToken(jsontext.TBeginObject)
	enc.WriteString("to")
	enc.WriteInt(int64(m.X))
	return enc.WriteToken(jsontext.TEndObject)
}

func marshalEdgeValues() []any {
	n := 7
	now := time.Date(2024, 5, 6, 7, 8, 9, 10, time.UTC)
	var nilMarshaler alosjson.Marshaler
	var nilStringer fmt.Stringer
	return []any{
		nil, true, false, 0, -1, int8(-128), uint64(math.MaxUint64), 1.5, float32(0.1), math.Copysign(0, -1), 1e21, 1e-7, 123456789.125,
		"", "plain", "quote\"back\\slash\nnl\ttab\x01\x1f", "<html>&amp;</html>", "日本語 😀", "\u2028\u2029",
		[]int(nil), []int{}, []int{1, 2, 3}, [3]string{"a", "b"}, []byte(nil), []byte{}, []byte("hello"),
		map[string]int(nil), map[string]int{}, map[string]int{"b": 2, "a": 1, "c": 3},
		map[int]string{10: "ten", 2: "two", -1: "neg"}, map[uint8]bool{1: true},
		map[textKey]int{{K: "x"}: 1}, map[string]any{"z": []any{1.0, "s", nil, map[string]any{}}, "a": nil},
		[]any(nil), []any{}, []any{nil, true, 2.5, "x", []any{}, map[string]any{"k": "v"}},
		&n, (*int)(nil), (**int)(nil), new(*int),
		PointerShaped{P: &n}, PointerShaped{}, MapShaped{}, NestedPointerShaped{},
		EmbeddedOuter{EmbeddedInner{A: 1, S: "s"}, 2}, EmbeddedPtrOuter{nil, 3}, EmbeddedPtrOuter{&EmbeddedInner{A: 4}, 5},
		EmbeddedNamed{}, EmbeddedUnexported{embeddedLower{L: 9}, 1}, EmbeddedConflict{}, EmbeddedShadow{},
		omitTarget{}, omitTarget{A: 1, B: "b", C: &n, D: []int{1}, E: map[string]int{"x": 1}, F: 2.5, G: &n, H: true, I: "i", L: 3, T: now, TP: &now, S: stringerImpl{1}, R: jsontext.Value(`{"raw":true}`)},
		&omitTarget{TP: nil},
		now, &now, (*time.Time)(nil), []*time.Time{nil, &now},
		valueMarshaler{X: 1}, &valueMarshaler{X: 2}, (*valueMarshaler)(nil), ptrMarshaler{X: 3}, &ptrMarshaler{X: 4}, (*ptrMarshaler)(nil),
		[]ptrMarshaler{{X: 5}}, map[string]ptrMarshaler{"k": {X: 6}}, marshalerToImpl{X: 7}, []marshalerToImpl{{X: 8}},
		struct{ M alosjson.Marshaler }{M: nilMarshaler}, struct{ M alosjson.Marshaler }{M: valueMarshaler{X: 9}},
		struct{ S fmt.Stringer }{S: nilStringer}, struct{ S fmt.Stringer }{S: stringerImpl{N: 2}},
		map[string]fmt.Stringer{"s": stringerImpl{N: 3}, "nil": nil},
		Level(1), []Level{1, 2}, map[string]Level{"l": 1}, Upper("abc"), struct{ U []Upper }{U: []Upper{"q"}},
		struct {
			A any
			B any
		}{A: PointerShaped{P: &n}, B: &EmbeddedOuter{}},
		[]any{Level(5), Upper("x"), int16(3), uint(4), float32(1.25), []string{"x"}, map[string]string{"a": "b"}},
		struct {
			Tagged string `json:"needs \"escaping\""`
		}{Tagged: "v"},
	}
}

func checkMarshalDifferential(t *testing.T, v any, opts []alosjson.Option) {
	t.Helper()
	want, wantErr := alosjson.MarshalReference(v, opts...)
	got, gotErr := alosjson.Marshal(v, opts...)
	if (wantErr == nil) != (gotErr == nil) {
		t.Fatalf("Marshal(%#v): reference err=%v, fast err=%v", v, wantErr, gotErr)
	}
	if wantErr == nil && !bytes.Equal(got, want) {
		t.Fatalf("Marshal(%#v) opts=%d:\nreference: %s\nfast:      %s", v, len(opts), want, got)
	}
	appended, appendErr := alosjson.MarshalAppend([]byte("prefix"), v, opts...)
	refAppended, refAppendErr := alosjson.MarshalAppendReference([]byte("prefix"), v, opts...)
	if (appendErr == nil) != (refAppendErr == nil) || appendErr == nil && !bytes.Equal(appended, refAppended) {
		t.Fatalf("MarshalAppend(%#v): reference %q/%v, fast %q/%v", v, refAppended, refAppendErr, appended, appendErr)
	}
}

func checkMarshalSemantic(t *testing.T, v any) {
	t.Helper()
	want, wantErr := alosjson.MarshalReference(v)
	got, gotErr := alosjson.Marshal(v)
	if (wantErr == nil) != (gotErr == nil) {
		t.Fatalf("Marshal(%#v): reference err=%v, fast err=%v", v, wantErr, gotErr)
	}
	if wantErr != nil {
		return
	}
	var a, b any
	if err := stdjson.Unmarshal(want, &a); err != nil {
		t.Fatalf("reference output invalid: %s", want)
	}
	if err := stdjson.Unmarshal(got, &b); err != nil {
		t.Fatalf("fast output invalid: %s", got)
	}
	if !reflect.DeepEqual(a, b) {
		t.Fatalf("semantic mismatch:\nreference: %s\nfast:      %s", want, got)
	}
}

func TestMarshalDifferentialEdgeValues(t *testing.T) {
	values := marshalEdgeValues()
	nan := []any{math.NaN(), math.Inf(1), float32(math.Inf(-1)), []float64{math.NaN()}, map[string]any{"x": math.Inf(1)}, struct{ F float64 }{math.NaN()}, struct {
		F float64 `json:",string"`
	}{math.NaN()}}
	unsupported := []any{make(chan int), func() {}, complex(1, 2), struct{ C chan int }{}}
	for _, v := range append(append(values, nan...), unsupported...) {
		for _, opts := range marshalOptionSets {
			checkMarshalDifferential(t, v, opts)
		}
		checkMarshalSemantic(t, v)
		checkMarshalDifferential(t, &v, marshalOptionSets[0])
	}
}

func TestMarshalDifferentialDecodedValues(t *testing.T) {
	count := 0
	for _, seed := range diffSeeds {
		for _, doc := range mutations(seed) {
			for _, target := range diffTargets {
				v := target.make()
				if alosjson.Unmarshal([]byte(doc), v) != nil {
					continue
				}
				checkMarshalDifferential(t, v, marshalOptionSets[0])
				checkMarshalDifferential(t, reflect.ValueOf(v).Elem().Interface(), marshalOptionSets[1])
				count++
			}
		}
	}
	t.Logf("compared %d decoded values", count)
}

func FuzzMarshalDifferential(f *testing.F) {
	for _, s := range diffSeeds {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, target := range diffTargets {
			v := target.make()
			if alosjson.Unmarshal(data, v) != nil {
				continue
			}
			for _, opts := range marshalOptionSets {
				checkMarshalDifferential(t, v, opts)
			}
		}
	})
}
