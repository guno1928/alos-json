package json_test

import (
	"bytes"
	stdjson "encoding/json"
	"io"
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	alosjson "github.com/guno1928/alos-json"
)

type mapSmall struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

type mapLarge struct {
	A, B, C, D, E, F, G, H [4]int64
	Label                  string
}

type mapNamedString string

type mapNamedInt int32

type mapCountingMarshaler struct{ Calls int }

func (m *mapCountingMarshaler) MarshalJSON() ([]byte, error) {
	m.Calls++
	return strconv.AppendInt(nil, int64(m.Calls), 10), nil
}

type mapHolder struct {
	Inner mapCountingMarshaler
}

func decodeObject(t *testing.T, data []byte) map[string]stdjson.RawMessage {
	t.Helper()
	var out map[string]stdjson.RawMessage
	if err := stdjson.Unmarshal(data, &out); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
	return out
}

var stdLikeOptions = []alosjson.Option{alosjson.EscapeHTML(true), alosjson.FormatNilSliceAsNull(true), alosjson.FormatNilMapAsNull(true)}

func decodeJSONValue(t *testing.T, data []byte) any {
	t.Helper()
	dec := stdjson.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var out any
	if err := dec.Decode(&out); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
	return out
}

type jsonScope struct {
	keys      map[string]bool
	expectKey bool
}

func requireNoDuplicateKeys(t *testing.T, data []byte) {
	t.Helper()
	dec := stdjson.NewDecoder(bytes.NewReader(data))
	var stack []*jsonScope
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return
		}
		if err != nil {
			t.Fatalf("tokenize %s: %v", data, err)
		}
		var top *jsonScope
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		if delim, ok := tok.(stdjson.Delim); ok && (delim == '}' || delim == ']') {
			stack = stack[:len(stack)-1]
			continue
		}
		if top != nil && top.keys != nil && top.expectKey {
			key := tok.(string)
			if top.keys[key] {
				t.Fatalf("duplicate key %q in %s", key, data)
			}
			top.keys[key], top.expectKey = true, false
			continue
		}
		if top != nil && top.keys != nil {
			top.expectKey = true
		}
		switch tok {
		case stdjson.Delim('{'):
			stack = append(stack, &jsonScope{keys: map[string]bool{}, expectKey: true})
		case stdjson.Delim('['):
			stack = append(stack, &jsonScope{})
		}
	}
}

func requireMapMatchesStd(t *testing.T, v any) {
	t.Helper()
	want, wantErr := stdjson.Marshal(v)
	got, gotErr := alosjson.Marshal(v, stdLikeOptions...)
	sorted, sortedErr := alosjson.Marshal(v, append(stdLikeOptions, alosjson.Deterministic(true))...)
	if (wantErr != nil) != (gotErr != nil) || (wantErr != nil) != (sortedErr != nil) {
		t.Fatalf("Marshal(%v): std err=%v, alos err=%v, alos sorted err=%v (alos out %s)", v, wantErr, gotErr, sortedErr, got)
	}
	if wantErr != nil {
		return
	}
	if !bytes.Equal(sorted, want) {
		t.Fatalf("Deterministic Marshal(%v) = %s; std = %s", v, sorted, want)
	}
	requireNoDuplicateKeys(t, got)
	if !reflect.DeepEqual(decodeJSONValue(t, got), decodeJSONValue(t, want)) {
		t.Fatalf("Marshal(%v) = %s, std = %s", v, got, want)
	}
}

func fillMap[K comparable, V any](n int, key func(int) K, val func(int) V) map[K]V {
	m := make(map[K]V, n)
	for i := range n {
		m[key(i)] = val(i)
	}
	return m
}

func TestLinkedMapEncodingMatchesStd(t *testing.T) {
	str := func(i int) string { return "v<" + strconv.Itoa(i) + ">\"&" }
	for _, n := range []int{0, 1, 7, 8, 9, 64, 1000} {
		cases := []any{
			fillMap(n, func(i int) int { return i*37 - 500 }, str),
			fillMap(n, func(i int) int8 { return int8(i) }, func(i int) int { return i }),
			fillMap(n, func(i int) int16 { return int16(i * -3) }, str),
			fillMap(n, func(i int) int32 { return int32(i) * math.MaxInt16 }, str),
			fillMap(n, func(i int) int64 { return int64(i) * math.MaxInt32 * 7 }, func(i int) any { return i }),
			fillMap(n, func(i int) uint { return uint(i) }, str),
			fillMap(n, func(i int) uint8 { return uint8(i) }, str),
			fillMap(n, func(i int) uint16 { return uint16(i) * 251 }, str),
			fillMap(n, func(i int) uint32 { return uint32(i) * 16777619 }, str),
			fillMap(n, func(i int) uint64 { return math.MaxUint64 - uint64(i) }, str),
			fillMap(n, func(i int) uintptr { return uintptr(i) }, str),
			fillMap(n, func(i int) mapNamedInt { return mapNamedInt(-i) }, str),
			fillMap(n, func(i int) mapNamedString { return mapNamedString("k" + strconv.Itoa(i)) }, str),
			fillMap(n, strconv.Itoa, func(i int) float64 { return float64(i) / 7 }),
			fillMap(n, strconv.Itoa, func(i int) bool { return i%2 == 0 }),
			fillMap(n, strconv.Itoa, func(i int) int64 { return int64(i) << 40 }),
			fillMap(n, strconv.Itoa, func(i int) mapSmall { return mapSmall{int64(i), str(i)} }),
			fillMap(n, strconv.Itoa, func(i int) *mapSmall {
				if i%3 == 0 {
					return nil
				}
				return &mapSmall{int64(i), str(i)}
			}),
			fillMap(n, strconv.Itoa, func(i int) mapLarge { return mapLarge{A: [4]int64{int64(i)}, Label: str(i)} }),
			fillMap(n, strconv.Itoa, func(i int) []string { return []string{str(i), "x"} }),
			fillMap(n, strconv.Itoa, func(i int) map[int]string { return map[int]string{i: str(i)} }),
			fillMap(n, strconv.Itoa, func(i int) time.Time { return time.Unix(int64(i)*86400, int64(i)) }),
			fillMap(n, func(i int) fixTextKey { return fixTextKey{i} }, str),
			fillMap(n, func(i int) [2]int { return [2]int{i, i} }, str),
		}
		for _, v := range cases {
			requireMapMatchesStd(t, v)
			requireMapMatchesStd(t, []any{v})
			requireMapMatchesStd(t, struct{ M any }{v})
		}
	}
}

func TestLinkedMapFloatAndBoolKeysMatchSortedPath(t *testing.T) {
	for _, v := range []any{
		fillMap(50, func(i int) float64 { return float64(i)/3 - 7 }, strconv.Itoa),
		fillMap(50, func(i int) float32 { return float32(i) * 1e30 }, strconv.Itoa),
		map[bool]int{true: 1, false: 0},
	} {
		got, err := alosjson.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		sorted, err := alosjson.Marshal(v, alosjson.Deterministic(true))
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(decodeObject(t, got), decodeObject(t, sorted)) {
			t.Fatalf("unsorted %s, sorted %s", got, sorted)
		}
	}
}

func TestLinkedMapNilAndEmpty(t *testing.T) {
	var nilMap map[int]string
	for _, tc := range []struct {
		v    any
		want string
	}{
		{nilMap, "{}"},
		{map[int]string{}, "{}"},
		{[]map[int]string{{}, nil, {1: "a"}}, `[{},{},{"1":"a"}]`},
		{struct{ A, B map[uint8]bool }{B: map[uint8]bool{}}, `{"A":{},"B":{}}`},
	} {
		got, err := alosjson.Marshal(tc.v)
		if err != nil || string(got) != tc.want {
			t.Fatalf("Marshal(%#v) = %s, %v; want %s", tc.v, got, err, tc.want)
		}
	}
	got, err := alosjson.Marshal(nilMap, alosjson.FormatNilMapAsNull(true))
	if err != nil || string(got) != "null" {
		t.Fatalf("FormatNilMapAsNull: %s, %v", got, err)
	}
}

func TestLinkedMapDoesNotMutateValuesWithPointerMethods(t *testing.T) {
	values := map[string]mapCountingMarshaler{"a": {}, "b": {}}
	nested := map[int]mapHolder{1: {}, 2: {}}
	for range 3 {
		if _, err := alosjson.Marshal(values); err != nil {
			t.Fatal(err)
		}
		if _, err := alosjson.Marshal(nested); err != nil {
			t.Fatal(err)
		}
	}
	for k, v := range values {
		if v.Calls != 0 {
			t.Fatalf("map value %q was mutated in place: %+v", k, v)
		}
	}
	for k, v := range nested {
		if v.Inner.Calls != 0 {
			t.Fatalf("nested map value %d was mutated in place: %+v", k, v)
		}
	}
}

func TestLinkedMapPropagatesErrors(t *testing.T) {
	for _, v := range []any{
		map[int]float64{1: 1, 2: math.NaN(), 3: 3},
		map[string]any{"a": map[uint16]any{7: math.Inf(1)}},
		map[int]*fixFailingTextKey{1: {}},
		map[[2]int]chan int{{1, 2}: nil},
	} {
		_, wantErr := stdjson.Marshal(v)
		if _, err := alosjson.Marshal(v); (err != nil) != (wantErr != nil) {
			t.Fatalf("Marshal(%#v): alos err %v, std err %v", v, err, wantErr)
		}
	}
}

func TestLinkedMapEscapesKeys(t *testing.T) {
	m := map[mapNamedString]int{"<a&b>": 1}
	want, _ := stdjson.Marshal(m)
	got, err := alosjson.Marshal(m, alosjson.EscapeHTML(true))
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("EscapeHTML key: %s, %v; want %s", got, err, want)
	}
}

func TestLinkedMapConcurrentReaders(t *testing.T) {
	m := fillMap(256, func(i int) uint32 { return uint32(i) }, func(i int) mapSmall { return mapSmall{int64(i), strconv.Itoa(i)} })
	want, _ := alosjson.Marshal(m, alosjson.Deterministic(true))
	done := make(chan error, 8)
	for range 8 {
		go func() {
			for range 50 {
				got, err := alosjson.Marshal(m)
				if err == nil && !reflect.DeepEqual(decodeObject(t, got), decodeObject(t, want)) {
					err = strconv.ErrSyntax
				}
				if err != nil {
					done <- err
					return
				}
			}
			done <- nil
		}()
	}
	for range 8 {
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}
}

func TestLinkedMapAllocations(t *testing.T) {
	if instrumentedBuild {
		t.Skip("allocation counts are not meaningful under race, asan or msan instrumentation")
	}
	if !alosjson.RuntimeMapHooks {
		t.Skip("this build encodes maps through reflection, which allocates per map")
	}
	m := fillMap(64, func(i int) int { return i }, strconv.Itoa)
	buf := make([]byte, 0, 4096)
	allocs := testing.AllocsPerRun(allocRuns, func() {
		buf, _ = alosjson.MarshalAppend(buf[:0], m)
	})
	if allocs > 2 {
		t.Fatalf("MarshalAppend(map[int]string) allocated %v times, want at most 2", allocs)
	}
}

func FuzzLinkedMapDifferential(f *testing.F) {
	f.Add([]byte("seed"), int64(0), 1.5)
	f.Add([]byte("\x00\xff<>&\"\\"), int64(-1), math.MaxFloat64)
	f.Add([]byte{}, int64(math.MinInt64), math.Copysign(0, -1))
	f.Fuzz(func(t *testing.T, raw []byte, base int64, scale float64) {
		ints := make(map[int64]string)
		floats := make(map[string]float64)
		structs := make(map[uint16]mapSmall)
		for i, c := range raw {
			k := base + int64(c)*int64(i+1)
			head, tail := strings.ToValidUTF8(string(raw[:i]), "?"), strings.ToValidUTF8(string(raw[i:]), "?")
			ints[k] = head
			floats[tail] = scale * float64(c)
			structs[uint16(k)] = mapSmall{ID: k, Name: tail}
		}
		requireMapMatchesStd(t, ints)
		requireMapMatchesStd(t, floats)
		requireMapMatchesStd(t, structs)
	})
}

func requireMapRoundTrip[M any](t *testing.T, original M) {
	t.Helper()
	data, err := stdjson.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	for _, got := range decodeBothPaths[M](t, string(data)) {
		if !reflect.DeepEqual(got, original) {
			t.Fatalf("decode %s into %T = %v, want %v", data, got, got, original)
		}
	}
}

func TestLinkedMapDecodeRoundTrip(t *testing.T) {
	str := func(i int) string { return "v" + strconv.Itoa(i) }
	for _, n := range []int{1, 7, 8, 9, 16, 17, 64, 1000} {
		requireMapRoundTrip(t, fillMap(n, strconv.Itoa, func(i int) bool { return i%3 == 0 }))
		requireMapRoundTrip(t, fillMap(n, strconv.Itoa, func(i int) int64 { return int64(i) * -77 }))
		requireMapRoundTrip(t, fillMap(n, strconv.Itoa, str))
		requireMapRoundTrip(t, fillMap(n, strconv.Itoa, func(i int) float64 { return float64(i) / 8 }))
		requireMapRoundTrip(t, fillMap(n, func(i int) uint64 { return math.MaxUint64 - uint64(i)*3 }, str))
		requireMapRoundTrip(t, fillMap(n, func(i int) int { return math.MinInt64 + i }, func(i int) bool { return i%2 == 1 }))
		requireMapRoundTrip(t, fillMap(n, func(i int) int64 { return math.MaxInt64 - int64(i) }, func(i int) int64 { return int64(i) }))
		requireMapRoundTrip(t, fillMap(n, func(i int) int { return i*37 - 900 }, str))
		requireMapRoundTrip(t, fillMap(n, func(i int) int64 { return int64(i) << 40 }, func(i int) float64 { return float64(i) / 4 }))
		requireMapRoundTrip(t, fillMap(n, func(i int) uint64 { return math.MaxUint64 - uint64(i) }, func(i int) mapSmall { return mapSmall{int64(i), str(i)} }))
		requireMapRoundTrip(t, fillMap(n, func(i int) int32 { return int32(-i) }, str))
		requireMapRoundTrip(t, fillMap(n, func(i int) uint32 { return uint32(i) * 7919 }, func(i int) int { return i }))
		requireMapRoundTrip(t, fillMap(n, func(i int) int16 { return int16(i) }, str))
		requireMapRoundTrip(t, fillMap(n, func(i int) uint8 { return uint8(i) }, str))
		requireMapRoundTrip(t, fillMap(n, func(i int) mapNamedInt { return mapNamedInt(i) }, str))
		requireMapRoundTrip(t, fillMap(n, func(i int) mapNamedString { return mapNamedString(str(i)) }, func(i int) []string { return []string{str(i)} }))
		requireMapRoundTrip(t, fillMap(n, strconv.Itoa, func(i int) *mapSmall { return &mapSmall{int64(i), str(i)} }))
		requireMapRoundTrip(t, fillMap(n, strconv.Itoa, func(i int) mapLarge { return mapLarge{A: [4]int64{int64(i)}, Label: str(i)} }))
		requireMapRoundTrip(t, fillMap(n, func(i int) int { return i }, func(i int) mapLarge { return mapLarge{H: [4]int64{3: int64(i)}, Label: str(i)} }))
		requireMapRoundTrip(t, fillMap(n, strconv.Itoa, func(i int) any { return str(i) }))
	}
}

func TestLinkedMapDecodeFloatAndBoolKeys(t *testing.T) {
	for _, got := range decodeBothPaths[map[float64]string](t, `{"1.5":"a","-2":"b","1e3":"c"}`) {
		if want := map[float64]string{1.5: "a", -2: "b", 1000: "c"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	for _, got := range decodeBothPaths[map[bool]int](t, `{"true":1,"false":2}`) {
		if want := map[bool]int{true: 1, false: 2}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestLinkedMapDecodeIntoExistingMap(t *testing.T) {
	m := map[int]mapSmall{1: {ID: 1, Name: "keep"}, 2: {ID: 2, Name: "old"}}
	if err := alosjson.Unmarshal([]byte(`{"2":{"name":"new"},"3":{"id":3}}`), &m); err != nil {
		t.Fatal(err)
	}
	want := map[int]mapSmall{1: {ID: 1, Name: "keep"}, 2: {Name: "new"}, 3: {ID: 3}}
	if !reflect.DeepEqual(m, want) {
		t.Fatalf("got %v, want %v", m, want)
	}
}

func TestLinkedMapDecodeErrorKeepsPreviousValue(t *testing.T) {
	for _, input := range []string{`{"b":{"id":2},"a":{"id":[1]}}`, `{"b":{"id":2},"a":{"id":1,"name":}}`} {
		m := map[string]mapSmall{"a": {ID: 1, Name: "before"}}
		if err := alosjson.Unmarshal([]byte(input), &m); err == nil {
			t.Fatalf("Unmarshal(%s) succeeded, want error", input)
		}
		if got := m["a"]; got != (mapSmall{ID: 1, Name: "before"}) {
			t.Fatalf("Unmarshal(%s) changed m[a] to %+v on error", input, got)
		}
		if got := m["b"]; got != (mapSmall{ID: 2}) {
			t.Fatalf("Unmarshal(%s) lost m[b]: %+v", input, got)
		}
	}
}

func requireSameKeyError[M any](t *testing.T, input string) {
	t.Helper()
	var fast, reflective M
	errFast := alosjson.Unmarshal([]byte(input), &fast)
	errReflect := alosjson.UnmarshalRead(strings.NewReader(input), &reflective)
	if errFast == nil || errReflect == nil {
		t.Fatalf("decode %s into %T: Unmarshal err=%v, UnmarshalRead err=%v, want both to fail", input, fast, errFast, errReflect)
	}
	if errFast.Error() != errReflect.Error() {
		t.Fatalf("decode %s into %T: Unmarshal err %q, UnmarshalRead err %q", input, fast, errFast, errReflect)
	}
}

func TestNativeIntKeyMapErrors(t *testing.T) {
	for _, key := range []string{"abc", "1.5", "1e3", "", " 1", "0x10", "99999999999999999999", "--1"} {
		input := `{"` + key + `":"v"}`
		requireSameKeyError[map[int]string](t, input)
		requireSameKeyError[map[int64]bool](t, input)
		requireSameKeyError[map[uint64]float64](t, input)
	}
	requireSameKeyError[map[uint64]int](t, `{"-1":1}`)
	requireSameKeyError[map[int64]int](t, `{"9223372036854775808":1}`)
	for _, got := range decodeBothPaths[map[int]int](t, `{"-0":1,"007":2}`) {
		if want := map[int]int{0: 1, 7: 2}; !reflect.DeepEqual(got, want) {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
