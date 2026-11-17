package json_test

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strings"
	"testing"

	alosjson "github.com/guno1928/alos-json"
	"github.com/guno1928/alos-json/jsontext"
)

type fixTextKey struct{ N int }

func (k fixTextKey) MarshalText() ([]byte, error) { return fmt.Appendf(nil, "text-%d", k.N), nil }

func (k *fixTextKey) UnmarshalText(b []byte) error {
	_, err := fmt.Sscanf(string(b), "text-%d", &k.N)
	return err
}

type fixPtrOnlyTextKey struct{ N int }

func (k *fixPtrOnlyTextKey) MarshalText() ([]byte, error) { return []byte("never"), nil }

type fixIntTextKey int

func (k fixIntTextKey) MarshalText() ([]byte, error) { return fmt.Appendf(nil, "ik-%d", int(k)), nil }

func (k *fixIntTextKey) UnmarshalText(b []byte) error {
	var n int
	if _, err := fmt.Sscanf(string(b), "ik-%d", &n); err != nil {
		return err
	}
	*k = fixIntTextKey(n)
	return nil
}

type fixPtrTextKey struct{ N int }

func (k *fixPtrTextKey) MarshalText() ([]byte, error) { return fmt.Appendf(nil, "p-%d", k.N), nil }

type fixFailingTextKey struct{}

var errFixTextKey = errors.New("text key failure")

func (fixFailingTextKey) MarshalText() ([]byte, error) { return nil, errFixTextKey }

type fixInner struct {
	Street string `json:"street"`
	City   string `json:"city"`
}

type fixPtrEmbed struct {
	*fixInner
	Name string `json:"name"`
}

type fixDeepInner struct {
	Zip string `json:"zip"`
}

type fixMiddle struct {
	*fixDeepInner
	Town string `json:"town"`
}

type fixDeepPtrEmbed struct {
	*fixMiddle
	ID int `json:"id"`
}

type fixNumberHolder struct {
	Count  stdjson.Number   `json:"count"`
	Ptr    *stdjson.Number  `json:"ptr,omitempty"`
	List   []stdjson.Number `json:"list,omitempty"`
	Quoted stdjson.Number   `json:"quoted,string,omitempty"`
}

type fixBrokenMarshaler struct{ Out string }

func (m fixBrokenMarshaler) MarshalJSON() ([]byte, error) { return []byte(m.Out), nil }

type fixRawHolder struct {
	R stdjson.RawMessage `json:"r"`
}

func requireSameAsStd(t *testing.T, v any) {
	t.Helper()
	want, wantErr := stdjson.Marshal(v)
	got, gotErr := alosjson.Marshal(v, alosjson.Deterministic(true))
	if (wantErr != nil) != (gotErr != nil) {
		t.Fatalf("Marshal(%#v): std err=%v, alos err=%v (alos out %s)", v, wantErr, gotErr, got)
	}
	if wantErr == nil && !bytes.Equal(got, want) {
		t.Fatalf("Marshal(%#v) = %s, std = %s", v, got, want)
	}
}

func decodeBothPaths[T any](t *testing.T, input string) []T {
	t.Helper()
	var fast, reflective T
	errFast := alosjson.Unmarshal([]byte(input), &fast)
	errReflect := alosjson.UnmarshalRead(strings.NewReader(input), &reflective)
	if errFast != nil || errReflect != nil {
		t.Fatalf("decode %s: Unmarshal err=%v, UnmarshalRead err=%v", input, errFast, errReflect)
	}
	return []T{fast, reflective}
}

func requireDecodeErrorBothPaths[T any](t *testing.T, input string) {
	t.Helper()
	var fast, reflective T
	if err := alosjson.Unmarshal([]byte(input), &fast); err == nil {
		t.Errorf("Unmarshal(%s) into %T succeeded with %#v, want error", input, fast, fast)
	}
	if err := alosjson.UnmarshalRead(strings.NewReader(input), &reflective); err == nil {
		t.Errorf("UnmarshalRead(%s) into %T succeeded with %#v, want error", input, reflective, reflective)
	}
}

func TestMapKeyEncodingMatchesStd(t *testing.T) {
	requireSameAsStd(t, map[fixTextKey]int{{N: 3}: 1})
	requireSameAsStd(t, map[fixTextKey]int{{N: 3}: 1, {N: 4}: 2, {N: 10}: 3})
	requireSameAsStd(t, map[fixIntTextKey]int{5: 1, -7: 2})
	requireSameAsStd(t, map[*fixPtrTextKey]int{{N: 2}: 1})
	requireSameAsStd(t, map[*fixPtrTextKey]int{nil: 1})
	requireSameAsStd(t, map[fixPtrOnlyTextKey]int{{N: 1}: 1})
	requireSameAsStd(t, map[fixPtrOnlyTextKey]int{})
	requireSameAsStd(t, map[[2]int]int{{1, 2}: 3})
	requireSameAsStd(t, map[struct{ A int }]int{})
	requireSameAsStd(t, map[fixFailingTextKey]int{{}: 1})
	requireSameAsStd(t, struct {
		M map[fixTextKey]string `json:"m"`
	}{M: map[fixTextKey]string{{N: 1}: "a", {N: 2}: "b"}})
}

func TestMapKeyEncodingUnsorted(t *testing.T) {
	out, err := alosjson.Marshal(map[fixTextKey]int{{N: 3}: 1, {N: 4}: 2})
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != `{"text-3":1,"text-4":2}` && string(out) != `{"text-4":2,"text-3":1}` {
		t.Fatalf("got %s", out)
	}
	if _, err := alosjson.Marshal(map[fixFailingTextKey]int{{}: 1}); !errors.Is(err, errFixTextKey) {
		t.Fatalf("MarshalText error not propagated: %v", err)
	}
}

func TestMapKeyFloatAndBool(t *testing.T) {
	cases := []struct {
		v    any
		want string
	}{
		{map[float64]int{1.5: 1, -2: 2, 1e21: 3, 1e-7: 4}, `{"-2":2,"1.5":1,"1e+21":3,"1e-7":4}`},
		{map[float32]int{0.1: 1}, `{"0.1":1}`},
		{map[bool]int{true: 1, false: 0}, `{"false":0,"true":1}`},
	}
	for _, tc := range cases {
		got, err := alosjson.Marshal(tc.v, alosjson.Deterministic(true))
		if err != nil || string(got) != tc.want {
			t.Errorf("Marshal(%#v) = %s, %v; want %s", tc.v, got, err, tc.want)
		}
	}
	for _, v := range []any{map[float64]int{math.NaN(): 1}, map[float64]int{math.Inf(1): 1}} {
		if out, err := alosjson.Marshal(v); err == nil {
			t.Errorf("Marshal(%v) = %s, want error", v, out)
		}
		if out, err := alosjson.Marshal(v, alosjson.Deterministic(true)); err == nil {
			t.Errorf("Marshal(%v, Deterministic) = %s, want error", v, out)
		}
	}
}

func TestMapKeyDecoding(t *testing.T) {
	for _, got := range decodeBothPaths[map[float64]int](t, `{"1.5":1,"-2":2,"1e+21":3}`) {
		if want := map[float64]int{1.5: 1, -2: 2, 1e21: 3}; !reflect.DeepEqual(got, want) {
			t.Errorf("float keys = %v, want %v", got, want)
		}
	}
	for _, got := range decodeBothPaths[map[bool]int](t, `{"true":1,"false":0}`) {
		if want := map[bool]int{true: 1, false: 0}; !reflect.DeepEqual(got, want) {
			t.Errorf("bool keys = %v, want %v", got, want)
		}
	}
	for _, got := range decodeBothPaths[map[fixIntTextKey]int](t, `{"ik-5":1,"ik--7":2}`) {
		if want := map[fixIntTextKey]int{5: 1, -7: 2}; !reflect.DeepEqual(got, want) {
			t.Errorf("int text keys = %v, want %v", got, want)
		}
	}
	for _, got := range decodeBothPaths[map[fixTextKey]int](t, `{"text-3":1}`) {
		if want := map[fixTextKey]int{{N: 3}: 1}; !reflect.DeepEqual(got, want) {
			t.Errorf("text keys = %v, want %v", got, want)
		}
	}
	requireDecodeErrorBothPaths[map[bool]int](t, `{"yes":1}`)
	requireDecodeErrorBothPaths[map[bool]int](t, `{"1":1}`)
	requireDecodeErrorBothPaths[map[float64]int](t, `{"NaN":1}`)
	requireDecodeErrorBothPaths[map[float64]int](t, `{"0x1p-2":1}`)
	requireDecodeErrorBothPaths[map[float64]int](t, `{"1.":1}`)
	requireDecodeErrorBothPaths[map[[2]int]int](t, `{"a":1}`)
	requireDecodeErrorBothPaths[map[fixTextKey]int](t, `{"bad":1}`)
}

func TestMapKeyRoundTrip(t *testing.T) {
	in := map[float64]bool{0.1: true, 2.5e-8: false, -3: true}
	out, err := alosjson.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range decodeBothPaths[map[float64]bool](t, string(out)) {
		if !reflect.DeepEqual(got, in) {
			t.Errorf("round trip of %s = %v, want %v", out, got, in)
		}
	}
}

func TestEmbeddedUnexportedPointerEncode(t *testing.T) {
	requireSameAsStd(t, fixPtrEmbed{fixInner: &fixInner{City: "c"}, Name: "n"})
	requireSameAsStd(t, fixPtrEmbed{Name: "n"})
	requireSameAsStd(t, &fixPtrEmbed{fixInner: &fixInner{Street: "s", City: "c"}, Name: "n"})
	requireSameAsStd(t, []fixPtrEmbed{{fixInner: &fixInner{City: "a"}}, {Name: "b"}})
	requireSameAsStd(t, fixDeepPtrEmbed{fixMiddle: &fixMiddle{fixDeepInner: &fixDeepInner{Zip: "z"}, Town: "t"}, ID: 1})
	requireSameAsStd(t, fixDeepPtrEmbed{fixMiddle: &fixMiddle{Town: "t"}, ID: 1})
}

func TestEmbeddedUnexportedPointerDecode(t *testing.T) {
	for _, got := range decodeBothPaths[fixPtrEmbed](t, `{"city":"c","name":"n"}`) {
		if got.fixInner == nil || got.City != "c" || got.Street != "" || got.Name != "n" {
			t.Errorf("decoded %+v (inner %+v)", got, got.fixInner)
		}
	}
	for _, got := range decodeBothPaths[fixPtrEmbed](t, `{"name":"n"}`) {
		if got.fixInner != nil {
			t.Errorf("inner allocated without any of its fields: %+v", got.fixInner)
		}
	}
	for _, got := range decodeBothPaths[fixDeepPtrEmbed](t, `{"zip":"z","town":"t","id":1}`) {
		if got.fixMiddle == nil || got.fixDeepInner == nil || got.Zip != "z" || got.Town != "t" || got.ID != 1 {
			t.Errorf("decoded %+v", got)
		}
	}
	existing := &fixInner{Street: "keep"}
	v := fixPtrEmbed{fixInner: existing}
	if err := alosjson.Unmarshal([]byte(`{"city":"c"}`), &v); err != nil {
		t.Fatal(err)
	}
	if v.fixInner != existing || v.Street != "keep" || v.City != "c" {
		t.Errorf("existing embedded pointer not reused: %+v", v.fixInner)
	}
}

func TestEmbeddedUnexportedPointerRoundTrip(t *testing.T) {
	in := fixDeepPtrEmbed{fixMiddle: &fixMiddle{fixDeepInner: &fixDeepInner{Zip: "z"}, Town: "t"}, ID: 7}
	out, err := alosjson.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range decodeBothPaths[fixDeepPtrEmbed](t, string(out)) {
		if got.ID != 7 || got.fixMiddle == nil || got.Town != "t" || got.fixDeepInner == nil || got.Zip != "z" {
			t.Errorf("round trip of %s = %+v", out, got)
		}
	}
}

func TestStdNumberEncode(t *testing.T) {
	ptr := stdjson.Number("4.5")
	for _, s := range []string{"12.50", "", "0", "-0", "1e-7", "-1.5E+10", "123456789012345678901234567890", "abc", "01", "1.", " 1", "+1", "NaN", "1e", "-", "0x10"} {
		n := stdjson.Number(s)
		requireSameAsStd(t, n)
		requireSameAsStd(t, map[string]any{"n": n})
		requireSameAsStd(t, []any{n, 1})
		requireSameAsStd(t, fixNumberHolder{Count: n, Ptr: &ptr, List: []stdjson.Number{n, "2"}, Quoted: n})
		requireSameAsStd(t, []stdjson.Number{n, n})
		requireSameAsStd(t, map[string]stdjson.Number{"k": n})
	}
}

func TestStdNumberDecode(t *testing.T) {
	for _, got := range decodeBothPaths[fixNumberHolder](t, `{"count":12.50,"ptr":-3e2,"list":[1,"2.5",-0]}`) {
		if got.Count != "12.50" || got.Ptr == nil || *got.Ptr != "-3e2" || !reflect.DeepEqual(got.List, []stdjson.Number{"1", "2.5", "-0"}) {
			t.Errorf("decoded %+v", got)
		}
	}
	for _, got := range decodeBothPaths[fixNumberHolder](t, `{"count":"12.50"}`) {
		if got.Count != "12.50" {
			t.Errorf("quoted number decoded as %q", got.Count)
		}
	}
	for _, got := range decodeBothPaths[stdjson.Number](t, `1e400`) {
		if got != "1e400" {
			t.Errorf("out-of-range literal decoded as %q", got)
		}
	}
	for _, got := range decodeBothPaths[map[string]stdjson.Number](t, `{"a":7,"b":"8"}`) {
		if want := map[string]stdjson.Number{"a": "7", "b": "8"}; !reflect.DeepEqual(got, want) {
			t.Errorf("map decoded %v", got)
		}
	}
	for _, input := range []string{`{"count":"abc"}`, `{"count":""}`, `{"count":true}`, `{"count":[1]}`, `{"count":{}}`, `{"count":"1 "}`} {
		requireDecodeErrorBothPaths[fixNumberHolder](t, input)
	}
}

func TestStdNumberDecodeDoesNotAliasInput(t *testing.T) {
	input := []byte(`{"count":123456789,"list":[987654321]}`)
	var got fixNumberHolder
	if err := alosjson.Unmarshal(input, &got); err != nil {
		t.Fatal(err)
	}
	for i := range input {
		input[i] = 'x'
	}
	if got.Count != "123456789" || got.List[0] != "987654321" {
		t.Fatalf("decoded value changed with the input buffer: %+v", got)
	}
}

func TestMarshalJSONOutputValidated(t *testing.T) {
	for _, out := range []string{`{"a":`, ``, `{"a":1}x`, `[1,]`, `nul`, `{"a":1}{"b":2}`, `"unterminated`} {
		values := []any{fixBrokenMarshaler{Out: out}, []any{fixBrokenMarshaler{Out: out}}, stdjson.RawMessage(out), fixRawHolder{R: stdjson.RawMessage(out)}}
		if out != "" {
			values = append(values, jsontext.Value(out))
		}
		for _, v := range values {
			got, err := alosjson.Marshal(v)
			if err == nil {
				t.Errorf("Marshal(%#v) = %s, want error", v, got)
				continue
			}
			var se *alosjson.SemanticError
			if !errors.As(err, &se) {
				t.Errorf("Marshal(%#v) error %T %v is not a SemanticError", v, err, err)
			}
			var w bytes.Buffer
			if err := alosjson.MarshalWrite(&w, v); err == nil {
				t.Errorf("MarshalWrite(%#v) = %s, want error", v, w.Bytes())
			}
		}
	}
	for _, tc := range []struct {
		v    any
		want string
	}{
		{fixBrokenMarshaler{Out: `{ "a" : 1 }`}, `{ "a" : 1 }`},
		{fixRawHolder{R: stdjson.RawMessage(` [1, 2] `)}, `{"r": [1, 2] }`},
		{fixRawHolder{}, `{"r":null}`},
		{struct {
			V jsontext.Value `json:"v"`
		}{}, `{"v":null}`},
		{jsontext.Value(nil), `null`},
		{jsontext.Value(` {"a":1} `), ` {"a":1} `},
		{stdjson.RawMessage(`"s"`), `"s"`},
	} {
		got, err := alosjson.Marshal(tc.v)
		if err != nil || string(got) != tc.want {
			t.Errorf("Marshal(%#v) = %s, %v; want %s", tc.v, got, err, tc.want)
		}
	}
}

func FuzzStdNumberDifferential(f *testing.F) {
	for _, s := range []string{"12.50", "", "0", "-0", "1e-7", "01", "1.", "abc", `"12.5"`, `"abc"`, "null", "true", " 1 ", `"1"`, "1e400", "[1]"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		want, wantErr := stdjson.Marshal(stdjson.Number(s))
		got, gotErr := alosjson.Marshal(stdjson.Number(s))
		if (wantErr != nil) != (gotErr != nil) || (wantErr == nil && !bytes.Equal(got, want)) {
			t.Fatalf("Marshal(Number(%q)): std %s %v, alos %s %v", s, want, wantErr, got, gotErr)
		}
		var stdNum, fastNum, reflectNum stdjson.Number
		stdErr := stdjson.Unmarshal([]byte(s), &stdNum)
		fastErr := alosjson.Unmarshal([]byte(s), &fastNum)
		reflectErr := alosjson.UnmarshalRead(strings.NewReader(s), &reflectNum)
		for _, got := range []struct {
			path string
			num  stdjson.Number
			err  error
		}{{"Unmarshal", fastNum, fastErr}, {"UnmarshalRead", reflectNum, reflectErr}} {
			if (stdErr != nil) != (got.err != nil) || (stdErr == nil && got.num != stdNum) {
				t.Fatalf("%s(%q): std %q %v, alos %q %v", got.path, s, stdNum, stdErr, got.num, got.err)
			}
		}
	})
}
