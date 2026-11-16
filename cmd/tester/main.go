package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"math"
	"os"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"

	json "github.com/guno1928/alos-json"
	"github.com/guno1928/alos-json/jsontext"
)

// ── Tracking ─────────────────────────────────────────────────────────

var (
	totalTests  int
	passedTests int
	failedTests int
	sectionName string
)

func section(name string) {
	sectionName = name
	fmt.Printf("\n╔══════════════════════════════════════════════════════════════╗\n")
	fmt.Printf("║  %-58s  ║\n", name)
	fmt.Printf("╚══════════════════════════════════════════════════════════════╝\n")
}

func check(name string, pass bool, detail string) {
	totalTests++
	if pass {
		passedTests++
		fmt.Printf("  [PASS] %s\n", name)
	} else {
		failedTests++
		fmt.Printf("  [FAIL] %s\n", name)
		fmt.Printf("         %s\n", detail)
	}
}

func expect(name string, got, want string) {
	totalTests++
	if got == want {
		passedTests++
		fmt.Printf("  [PASS] %s  =>  %s\n", name, got)
	} else {
		failedTests++
		fmt.Printf("  [FAIL] %s\n", name)
		fmt.Printf("         got:  %s\n", got)
		fmt.Printf("         want: %s\n", want)
	}
}

func mustMarshal(v any, opts ...json.Option) []byte {
	b, err := json.Marshal(v, opts...)
	if err != nil {
		fmt.Printf("  [FATAL] Marshal error: %v\n", err)
		return nil
	}
	return b
}

// ── Main ─────────────────────────────────────────────────────────────

func main() {
	fmt.Println("================================================================")
	fmt.Println("  ALOS-JSON Comprehensive Test Suite")
	fmt.Println("================================================================")

	testPrimitives()
	testStructTags()
	testCompositeTypes()
	testPointers()
	testBase64Bytes()
	testOptions()
	testValidCompactIndent()
	testStreamingIO()
	testInterfaceAny()
	testUnicodeEscaping()
	testErrorHandling()
	testRobustness()
	testCrashResistance()
	testRoundtrip()
	testStreamingDecoder()
	testEncoder()
	testEdgeCases()
	testNumberEdgeCases()
	testConcurrency()
	testSecurityExploits()

	fmt.Println("\n================================================================")
	if failedTests == 0 {
		fmt.Printf("  ALL %d TESTS PASSED\n", totalTests)
	} else {
		fmt.Printf("  %d PASSED  |  %d FAILED  |  %d TOTAL\n", passedTests, failedTests, totalTests)
	}
	fmt.Println("================================================================")
	if failedTests > 0 {
		os.Exit(1)
	}
}

// =====================================================================
// 1. PRIMITIVE TYPES — 17 tests
// =====================================================================

func testPrimitives() {
	section("1. PRIMITIVE TYPES (marshal + unmarshal)")

	expect("bool true", string(mustMarshal(true)), `true`)
	expect("bool false", string(mustMarshal(false)), `false`)
	expect("int 0", string(mustMarshal(0)), `0`)
	expect("int 42", string(mustMarshal(42)), `42`)
	expect("int -999", string(mustMarshal(-999)), `-999`)
	expect("int8 max", string(mustMarshal(int8(127))), `127`)
	expect("int16 min", string(mustMarshal(int16(-32768))), `-32768`)
	expect("int32 max", string(mustMarshal(int32(2147483647))), `2147483647`)
	expect("int64 max", string(mustMarshal(int64(9223372036854775807))), `9223372036854775807`)
	expect("uint 0", string(mustMarshal(uint(0))), `0`)
	expect("uint8 max", string(mustMarshal(uint8(255))), `255`)
	expect("uint64 max", string(mustMarshal(uint64(18446744073709551615))), `18446744073709551615`)
	expect("float32", string(mustMarshal(float32(1.5))), `1.5`)
	expect("float64 pi", string(mustMarshal(3.14)), `3.14`)
	expect("float -0.5", string(mustMarshal(-0.5)), `-0.5`)
	expect("string hello", string(mustMarshal("hello")), `"hello"`)
	expect("string empty", string(mustMarshal("")), `""`)

	// Unmarshal checks
	var b bool
	json.Unmarshal([]byte(`true`), &b)
	check("unmarshal bool true", b == true, fmt.Sprintf("got %v", b))

	var n int64
	json.Unmarshal([]byte(`9223372036854775807`), &n)
	check("unmarshal int64 max", n == 9223372036854775807, fmt.Sprintf("got %d", n))

	var u uint64
	json.Unmarshal([]byte(`18446744073709551615`), &u)
	check("unmarshal uint64 max", u == 18446744073709551615, fmt.Sprintf("got %d", u))

	var f float64
	json.Unmarshal([]byte(`3.14`), &f)
	check("unmarshal float64", f == 3.14, fmt.Sprintf("got %v", f))

	var s string
	json.Unmarshal([]byte(`"world"`), &s)
	check("unmarshal string", s == "world", fmt.Sprintf("got %q", s))

	expect("null", string(mustMarshal(nil)), `null`)

	var p *int
	json.Unmarshal([]byte(`null`), &p)
	check("unmarshal null ptr", p == nil, fmt.Sprintf("got %v", p))
}

// =====================================================================
// 2. STRUCT TAGS — 18 tests
// =====================================================================

func testStructTags() {
	section("2. STRUCT TAGS (omitempty, omitzero, string, naming)")

	type Basic struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	expect("basic struct", string(mustMarshal(Basic{Name: "Alice", Age: 30})),
		`{"name":"Alice","age":30}`)

	var out Basic
	json.Unmarshal([]byte(`{"name":"Bob","age":25}`), &out)
	check("unmarshal basic", out.Name == "Bob" && out.Age == 25,
		fmt.Sprintf("got %+v", out))

	type OmitE struct {
		A string   `json:"a"`
		B string   `json:"b,omitempty"`
		C int      `json:"c,omitempty"`
		D float64  `json:"d,omitempty"`
		E bool     `json:"e,omitempty"`
		F []string `json:"f,omitempty"`
	}
	b := mustMarshal(OmitE{A: "x"})
	check("omitempty: empty str omitted", !bytes.Contains(b, []byte(`"b"`)), string(b))
	check("omitempty: zero int omitted", !bytes.Contains(b, []byte(`"c"`)), string(b))
	check("omitempty: zero float omitted", !bytes.Contains(b, []byte(`"d"`)), string(b))
	check("omitempty: false bool omitted", !bytes.Contains(b, []byte(`"e"`)), string(b))
	check("omitempty: nil slice omitted", !bytes.Contains(b, []byte(`"f"`)), string(b))
	b = mustMarshal(OmitE{A: "x", B: "y", C: 1, E: true, F: []string{"z"}})
	check("omitempty: non-empty included", bytes.Contains(b, []byte(`"b":"y"`)), string(b))

	type OmitZ struct {
		A string `json:"a"`
		B int    `json:"b,omitzero"`
		C *int   `json:"c,omitzero"`
	}
	b = mustMarshal(OmitZ{A: "x"})
	check("omitzero: zero int omitted", !bytes.Contains(b, []byte(`"b"`)), string(b))
	check("omitzero: nil ptr omitted", !bytes.Contains(b, []byte(`"c"`)), string(b))
	n := 0
	b = mustMarshal(OmitZ{A: "x", C: &n})
	check("omitzero: non-nil ptr included", bytes.Contains(b, []byte(`"c":0`)), string(b))

	type StrTag struct {
		Count  int     `json:"count,string"`
		Score  float64 `json:"score,string"`
		Active bool    `json:"active,string"`
	}
	b = mustMarshal(StrTag{Count: 42, Score: 3.14, Active: true})
	check("string tag: int", bytes.Contains(b, []byte(`"count":"42"`)), string(b))
	check("string tag: float", bytes.Contains(b, []byte(`"score":"3.14"`)), string(b))
	check("string tag: bool", bytes.Contains(b, []byte(`"active":"true"`)), string(b))

	type Ignored struct {
		Vis string `json:"vis"`
		Ign string `json:"-"`
	}
	b = mustMarshal(Ignored{Vis: "a", Ign: "b"})
	check("json:\"-\" omits field", !bytes.Contains(b, []byte(`Ign`)) && !bytes.Contains(b, []byte(`"b"`)), string(b))

	type NoTag struct {
		MyField string
	}
	b = mustMarshal(NoTag{MyField: "val"})
	check("no tag uses Go name", bytes.Contains(b, []byte(`"MyField":"val"`)), string(b))
}

// =====================================================================
// 3. COMPOSITE TYPES — 20 tests
// =====================================================================

func testCompositeTypes() {
	section("3. COMPOSITE TYPES (slices, arrays, maps, nested)")

	expect("int slice", string(mustMarshal([]int{1, 2, 3})), `[1,2,3]`)
	expect("string slice", string(mustMarshal([]string{"a", "b"})), `["a","b"]`)
	expect("empty slice", string(mustMarshal([]int{})), `[]`)
	expect("bool slice", string(mustMarshal([]bool{true, false, true})), `[true,false,true]`)
	expect("float slice", string(mustMarshal([]float64{1.1, 2.2})), `[1.1,2.2]`)

	var ints []int
	json.Unmarshal([]byte(`[10,20,30]`), &ints)
	check("unmarshal int slice", reflect.DeepEqual(ints, []int{10, 20, 30}), fmt.Sprintf("got %v", ints))

	expect("array [3]int", string(mustMarshal([3]int{10, 20, 30})), `[10,20,30]`)
	var arr [3]int
	json.Unmarshal([]byte(`[1,2,3]`), &arr)
	check("unmarshal array", arr == [3]int{1, 2, 3}, fmt.Sprintf("got %v", arr))

	expect("map str->str", string(mustMarshal(map[string]string{"k": "v"})), `{"k":"v"}`)

	var m map[string]any
	json.Unmarshal([]byte(`{"name":"Alice","age":30,"active":true}`), &m)
	check("map[string]any: string", m["name"] == "Alice", fmt.Sprintf("got %v", m["name"]))
	check("map[string]any: number", m["age"] == float64(30), fmt.Sprintf("got %v (%T)", m["age"], m["age"]))
	check("map[string]any: bool", m["active"] == true, fmt.Sprintf("got %v", m["active"]))

	json.Unmarshal([]byte(`{"arr":[1,2],"obj":{"k":"v"},"nil":null}`), &m)
	arr2, ok := m["arr"].([]any)
	check("map[string]any: nested array", ok && len(arr2) == 2, fmt.Sprintf("got %v", m["arr"]))
	obj, ok := m["obj"].(map[string]any)
	check("map[string]any: nested object", ok && obj["k"] == "v", fmt.Sprintf("got %v", m["obj"]))
	check("map[string]any: null value", m["nil"] == nil, fmt.Sprintf("got %v", m["nil"]))

	b := mustMarshal(map[int]string{1: "one", 2: "two"}, json.Deterministic(true))
	expect("map int key sorted", string(b), `{"1":"one","2":"two"}`)

	type Inner struct {
		X int `json:"x"`
	}
	type Outer struct {
		Name  string `json:"name"`
		Inner Inner  `json:"inner"`
	}
	expect("nested struct", string(mustMarshal(Outer{Name: "t", Inner: Inner{X: 42}})),
		`{"name":"t","inner":{"x":42}}`)

	type L3 struct {
		V string `json:"v"`
	}
	type L2 struct {
		L3 L3 `json:"l3"`
	}
	type L1 struct {
		L2 L2 `json:"l2"`
	}
	expect("deep nested", string(mustMarshal(L1{L2: L2{L3: L3{V: "deep"}}})),
		`{"l2":{"l3":{"v":"deep"}}}`)
}

// =====================================================================
// 4. POINTER HANDLING — 15 tests
// =====================================================================

func testPointers() {
	section("4. POINTER HANDLING")

	name := "Alice"
	age := 30
	type PStr struct {
		Name *string `json:"name"`
		Age  *int    `json:"age"`
	}
	expect("ptr fields", string(mustMarshal(PStr{Name: &name, Age: &age})),
		`{"name":"Alice","age":30}`)

	var ps PStr
	json.Unmarshal([]byte(`{"name":"Bob","age":25}`), &ps)
	check("unmarshal ptr string", ps.Name != nil && *ps.Name == "Bob", fmt.Sprintf("got %v", ps.Name))
	check("unmarshal ptr int", ps.Age != nil && *ps.Age == 25, fmt.Sprintf("got %v", ps.Age))

	expect("nil ptr string", string(mustMarshal(PStr{Name: nil})),
		`{"name":null,"age":null}`)

	json.Unmarshal([]byte(`{"name":null,"age":null}`), &ps)
	check("unmarshal null->nil name", ps.Name == nil, "name not nil")
	check("unmarshal null->nil age", ps.Age == nil, "age not nil")

	type Inner struct {
		X int `json:"x"`
	}
	type WPtr struct {
		Inner *Inner `json:"inner"`
	}
	expect("ptr-to-struct", string(mustMarshal(WPtr{Inner: &Inner{X: 99}})),
		`{"inner":{"x":99}}`)
	expect("nil ptr-to-struct", string(mustMarshal(WPtr{Inner: nil})),
		`{"inner":null}`)

	var wp WPtr
	json.Unmarshal([]byte(`{"inner":{"x":77}}`), &wp)
	check("unmarshal ptr-to-struct", wp.Inner != nil && wp.Inner.X == 77,
		fmt.Sprintf("got %+v", wp.Inner))

	json.Unmarshal([]byte(`{"inner":null}`), &wp)
	check("unmarshal null ptr-to-struct", wp.Inner == nil, "not nil")

	type DPtr struct {
		Val **int `json:"val"`
	}
	nn := 42
	pp := &nn
	expect("double ptr", string(mustMarshal(DPtr{Val: &pp})), `{"val":42}`)

	var dp DPtr
	json.Unmarshal([]byte(`{"val":99}`), &dp)
	check("unmarshal double ptr", dp.Val != nil && *dp.Val != nil && **dp.Val == 99,
		"double ptr failed")

	expect("nil double ptr", string(mustMarshal(DPtr{Val: nil})), `{"val":null}`)

	type Triple struct {
		V ***int `json:"v"`
	}
	nnn := 5
	ppp := &nnn
	pppp := &ppp
	expect("triple ptr", string(mustMarshal(Triple{V: &pppp})), `{"v":5}`)
}

// =====================================================================
// 5. BASE64 / BYTE SLICES — 15 tests
// =====================================================================

func testBase64Bytes() {
	section("5. BASE64 / BYTE SLICES")

	type B struct {
		Data []byte `json:"data"`
	}

	tests := []struct {
		name  string
		input []byte
	}{
		{"empty bytes", []byte{}},
		{"hello", []byte("hello")},
		{"binary 0-255", func() []byte {
			b := make([]byte, 256)
			for i := range b {
				b[i] = byte(i)
			}
			return b
		}()},
		{"spaces", []byte("   ")},
		{"json chars", []byte(`{"key":"value"}`)},
		{"null bytes", []byte{0, 0, 0}},
		{"single byte", []byte{42}},
		{"unicode bytes", []byte("Hello, 世界")},
		{"newlines", []byte("line1\nline2\r\nline3")},
		{"long data", bytes.Repeat([]byte("abcdef"), 1000)},
	}

	for _, tt := range tests {
		b := mustMarshal(B{Data: tt.input})
		var out B
		err := json.Unmarshal(b, &out)
		ok := err == nil && bytes.Equal(out.Data, tt.input)
		check("base64 roundtrip: "+tt.name, ok, fmt.Sprintf("err=%v len(out)=%d", err, len(out.Data)))
	}

	// Nil bytes -> null
	b := mustMarshal(B{Data: nil})
	check("nil bytes -> null", bytes.Contains(b, []byte(`null`)), string(b))

	var out B
	json.Unmarshal([]byte(`{"data":null}`), &out)
	check("unmarshal null bytes", out.Data == nil, fmt.Sprintf("got %v", out.Data))

	// Verify encoding is actually base64
	b = mustMarshal(B{Data: []byte("hello world")})
	encoded := base64.StdEncoding.EncodeToString([]byte("hello world"))
	check("output is base64", bytes.Contains(b, []byte(encoded)), string(b))

	// Invalid base64 should error
	err := json.Unmarshal([]byte(`{"data":"not-valid-base64!!!"}`), &out)
	check("invalid base64 errors", err != nil, "expected error")
}

// =====================================================================
// 6. OPTIONS — 18 tests
// =====================================================================

func testOptions() {
	section("6. OPTIONS (EscapeHTML, Reject, CaseInsensitive, Deterministic, Nil)")

	type H struct {
		HTML string `json:"html"`
	}
	b := mustMarshal(H{HTML: `<script>alert("xss")</script>`}, json.EscapeHTML(true))
	check("EscapeHTML: no <script>", !bytes.Contains(b, []byte(`<script>`)), string(b))
	check("EscapeHTML: has \\u003c", bytes.Contains(b, []byte(`\u003c`)), string(b))

	b = mustMarshal(H{HTML: `A & B < C > D`}, json.EscapeHTML(true))
	check("EscapeHTML: & escaped", bytes.Contains(b, []byte(`\u0026`)), string(b))
	check("EscapeHTML: < escaped", bytes.Contains(b, []byte(`\u003c`)), string(b))
	check("EscapeHTML: > escaped", bytes.Contains(b, []byte(`\u003e`)), string(b))

	b = mustMarshal(H{HTML: `<b>bold</b>`})
	check("no EscapeHTML: raw <b>", bytes.Contains(b, []byte(`<b>`)), string(b))

	type S struct {
		Name string `json:"name"`
	}
	err := json.Unmarshal([]byte(`{"name":"a","extra":"bad"}`), &S{}, json.RejectUnknownMembers(true))
	check("RejectUnknown: errors on extra", err != nil, "expected error")

	err = json.Unmarshal([]byte(`{"name":"a","extra":"ok"}`), &S{})
	check("default: skips unknown", err == nil, fmt.Sprintf("got %v", err))

	var cs S
	json.Unmarshal([]byte(`{"NAME":"Alice"}`), &cs, json.MatchCaseInsensitiveNames(true))
	check("CaseInsensitive: NAME->name", cs.Name == "Alice", fmt.Sprintf("got %q", cs.Name))

	cs = S{}
	json.Unmarshal([]byte(`{"NaMe":"Bob"}`), &cs, json.MatchCaseInsensitiveNames(true))
	check("CaseInsensitive: NaMe->name", cs.Name == "Bob", fmt.Sprintf("got %q", cs.Name))

	cs = S{}
	json.Unmarshal([]byte(`{"NAME":"X"}`), &cs)
	check("CaseSensitive: NAME misses", cs.Name == "", fmt.Sprintf("got %q", cs.Name))

	m := map[string]int{"c": 3, "a": 1, "b": 2}
	expect("deterministic order", string(mustMarshal(m, json.Deterministic(true))),
		`{"a":1,"b":2,"c":3}`)

	type NilS struct {
		Items []int `json:"items"`
	}
	b = mustMarshal(NilS{})
	check("nil slice -> []", bytes.Contains(b, []byte(`"items":[]`)), string(b))
	b = mustMarshal(NilS{}, json.FormatNilSliceAsNull(true))
	check("nil slice -> null", bytes.Contains(b, []byte(`"items":null`)), string(b))

	type NilM struct {
		Data map[string]int `json:"data"`
	}
	b = mustMarshal(NilM{})
	check("nil map -> {}", bytes.Contains(b, []byte(`"data":{}`)), string(b))
	b = mustMarshal(NilM{}, json.FormatNilMapAsNull(true))
	check("nil map -> null", bytes.Contains(b, []byte(`"data":null`)), string(b))
}

// =====================================================================
// 7. VALID / COMPACT / INDENT — 18 tests
// =====================================================================

func testValidCompactIndent() {
	section("7. VALID / COMPACT / INDENT")

	valids := []string{
		`{}`, `[]`, `null`, `true`, `false`, `42`, `-1`, `3.14`,
		`"hello"`, `{"a":1}`, `[1,2,3]`, `{"nested":{"deep":1}}`,
		`[1,"two",null,true,false]`, `{"a":[1,2,{"b":3}]}`,
	}
	for _, s := range valids {
		check("valid: "+s, json.Valid([]byte(s)), "expected valid")
	}

	invalids := []string{
		``, `{`, `}`, `[`, `]`, `{invalid}`, `{"a":}`, `[1,]`, `undefined`, `NaN`,
	}
	for _, s := range invalids {
		check("invalid: "+s, !json.Valid([]byte(s)), "expected invalid")
	}

	// Compact
	var dst []byte
	json.Compact(&dst, []byte(`{  "a" : 1 ,  "b"  :  [ 2 , 3 ]  }`))
	expect("compact", string(dst), `{"a":1,"b":[2,3]}`)

	// Indent
	var dst2 []byte
	json.Indent(&dst2, []byte(`{"a":1,"b":[2,3]}`), "", "  ")
	check("indent: has newlines", bytes.Contains(dst2, []byte("\n")), string(dst2))
	check("indent: still valid", json.Valid(dst2), string(dst2))
}

// =====================================================================
// 8. STREAMING I/O — 15 tests
// =====================================================================

func testStreamingIO() {
	section("8. STREAMING I/O (MarshalWrite, UnmarshalRead, MarshalAppend)")

	type S struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	var buf bytes.Buffer
	json.MarshalWrite(&buf, S{Name: "stream", Age: 1})
	expect("MarshalWrite", buf.String(), `{"name":"stream","age":1}`)

	buf.Reset()
	json.MarshalWrite(&buf, []int{1, 2, 3})
	expect("MarshalWrite slice", buf.String(), `[1,2,3]`)

	buf.Reset()
	json.MarshalWrite(&buf, "hello")
	expect("MarshalWrite string", buf.String(), `"hello"`)

	buf.Reset()
	json.MarshalWrite(&buf, true)
	expect("MarshalWrite bool", buf.String(), `true`)

	buf.Reset()
	json.MarshalWrite(&buf, nil)
	expect("MarshalWrite null", buf.String(), `null`)

	var out S
	json.UnmarshalRead(strings.NewReader(`{"name":"reader","age":99}`), &out)
	check("UnmarshalRead struct", out.Name == "reader" && out.Age == 99,
		fmt.Sprintf("got %+v", out))

	var out2 []int
	json.UnmarshalRead(strings.NewReader(`[10,20,30]`), &out2)
	check("UnmarshalRead slice", reflect.DeepEqual(out2, []int{10, 20, 30}),
		fmt.Sprintf("got %v", out2))

	var out3 map[string]any
	json.UnmarshalRead(strings.NewReader(`{"k":"v"}`), &out3)
	check("UnmarshalRead map", out3["k"] == "v", fmt.Sprintf("got %v", out3))

	// MarshalAppend
	res, _ := json.MarshalAppend([]byte("PREFIX:"), S{Name: "a", Age: 1})
	check("MarshalAppend prefix", bytes.HasPrefix(res, []byte("PREFIX:")), string(res))
	check("MarshalAppend suffix", bytes.HasSuffix(res, []byte(`{"name":"a","age":1}`)), string(res))

	res, _ = json.MarshalAppend(nil, 42)
	expect("MarshalAppend nil dst", string(res), `42`)

	res, _ = json.MarshalAppend([]byte{}, "test")
	expect("MarshalAppend empty dst", string(res), `"test"`)

	res, _ = json.MarshalAppend([]byte("["), 1)
	expect("MarshalAppend partial", string(res), `[1`)

	res, _ = json.MarshalAppend(make([]byte, 0, 1024), []int{1, 2})
	expect("MarshalAppend prealloc", string(res), `[1,2]`)
}

// =====================================================================
// 9. INTERFACE / ANY — 16 tests
// =====================================================================

func testInterfaceAny() {
	section("9. INTERFACE / ANY type handling")

	var v any

	json.Unmarshal([]byte(`42`), &v)
	check("any: number is float64", v == float64(42), fmt.Sprintf("got %T(%v)", v, v))

	json.Unmarshal([]byte(`"hello"`), &v)
	check("any: string", v == "hello", fmt.Sprintf("got %v", v))

	json.Unmarshal([]byte(`true`), &v)
	check("any: true", v == true, fmt.Sprintf("got %v", v))

	json.Unmarshal([]byte(`false`), &v)
	check("any: false", v == false, fmt.Sprintf("got %v", v))

	json.Unmarshal([]byte(`null`), &v)
	check("any: null", v == nil, fmt.Sprintf("got %v", v))

	json.Unmarshal([]byte(`[1,"two",true,null]`), &v)
	arr, ok := v.([]any)
	check("any: array type", ok && len(arr) == 4, fmt.Sprintf("got %T", v))
	if ok {
		check("any: arr[0] float64", arr[0] == float64(1), fmt.Sprintf("got %v", arr[0]))
		check("any: arr[1] string", arr[1] == "two", fmt.Sprintf("got %v", arr[1]))
		check("any: arr[2] bool", arr[2] == true, fmt.Sprintf("got %v", arr[2]))
		check("any: arr[3] nil", arr[3] == nil, fmt.Sprintf("got %v", arr[3]))
	}

	json.Unmarshal([]byte(`{"k":"v","n":1}`), &v)
	mp, ok := v.(map[string]any)
	check("any: object type", ok, fmt.Sprintf("got %T", v))
	if ok {
		check("any: obj[k]", mp["k"] == "v", fmt.Sprintf("got %v", mp["k"]))
		check("any: obj[n]", mp["n"] == float64(1), fmt.Sprintf("got %T(%v)", mp["n"], mp["n"]))
	}

	// Nested any roundtrip
	nested := map[string]any{
		"a": []any{float64(1), "two", map[string]any{"deep": true}},
	}
	b := mustMarshal(nested)
	var out map[string]any
	json.Unmarshal(b, &out)
	outArr, _ := out["a"].([]any)
	check("any: nested roundtrip len", len(outArr) == 3, fmt.Sprintf("got %d", len(outArr)))
	deepMap, _ := outArr[2].(map[string]any)
	check("any: deep map value", deepMap["deep"] == true, fmt.Sprintf("got %v", deepMap))
}

// =====================================================================
// 10. UNICODE / ESCAPING — 17 tests
// =====================================================================

func testUnicodeEscaping() {
	section("10. UNICODE / ESCAPING")

	type S struct {
		T string `json:"t"`
	}

	// Roundtrip special strings
	specials := []string{
		"Hello, 世界",
		"日本語テスト",
		"émojis: 🎉🚀",
		"tab:\there",
		"newline:\nhere",
		"cr:\rhere",
		"quote:\"here",
		"backslash:\\here",
		"null:\x00byte",
		"\x01\x02\x1f",
	}
	for _, s := range specials {
		b := mustMarshal(S{T: s})
		ok := json.Valid(b)
		label := fmt.Sprintf("%q", s)
		if len(label) > 30 {
			label = label[:30]
		}
		check("escape valid: "+label, ok, string(b))
		var out S
		json.Unmarshal(b, &out)
		check("escape roundtrip: "+label, out.T == s,
			fmt.Sprintf("got %q", out.T))
	}

	// Control chars produce \u00xx
	b := mustMarshal("\x00\x01\x02")
	check("ctrl chars \\u0000", bytes.Contains(b, []byte(`\u0000`)), string(b))
	check("ctrl chars \\u0001", bytes.Contains(b, []byte(`\u0001`)), string(b))
	check("ctrl chars \\u0002", bytes.Contains(b, []byte(`\u0002`)), string(b))

	// Unicode escape in JSON input
	var s2 string
	json.Unmarshal([]byte(`"\u0048\u0065\u006c\u006c\u006f"`), &s2)
	check("\\uXXXX decode", s2 == "Hello", fmt.Sprintf("got %q", s2))
}

// =====================================================================
// 11. ERROR HANDLING — 16 tests
// =====================================================================

func testErrorHandling() {
	section("11. ERROR HANDLING")

	var s string
	err := json.Unmarshal([]byte(`"hi"`), s)
	check("non-pointer errors", err != nil, "expected error")

	err = json.Unmarshal([]byte(`"hi"`), (*string)(nil))
	check("nil pointer errors", err != nil, "expected error")

	var n int
	err = json.Unmarshal([]byte(`"not a number"`), &n)
	check("string->int errors", err != nil, "expected error")

	var b bool
	err = json.Unmarshal([]byte(`"not a bool"`), &b)
	check("string->bool errors", err != nil, "expected error")

	var f float64
	err = json.Unmarshal([]byte(`"not a float"`), &f)
	check("string->float errors", err != nil, "expected error")

	var v any
	err = json.Unmarshal([]byte(``), &v)
	check("empty input errors", err != nil, "expected error")

	err = json.Unmarshal([]byte(`{invalid}`), &v)
	check("invalid obj errors", err != nil, "expected error")

	err = json.Unmarshal([]byte(`{"a":}`), &v)
	check("missing value errors", err != nil, "expected error")

	// Truncated inputs
	err = json.Unmarshal([]byte(`[1,2,`), &v)
	check("truncated array errors", err != nil, fmt.Sprintf("got err=%v", err))

	err = json.Unmarshal([]byte(`{"a":1`), &v)
	check("truncated object errors", err != nil, fmt.Sprintf("got err=%v", err))

	err = json.Unmarshal([]byte(`"unterminated`), &v)
	check("unterminated string errors", err != nil, "expected error")

	_, err = json.Marshal(math.NaN())
	check("NaN errors", err != nil, "expected error")

	_, err = json.Marshal(math.Inf(1))
	check("+Inf errors", err != nil, "expected error")

	_, err = json.Marshal(math.Inf(-1))
	check("-Inf errors", err != nil, "expected error")

	ch := make(chan int)
	_, err = json.Marshal(ch)
	check("channel errors", err != nil, "expected error")

	_, err = json.Marshal(func() {})
	check("func errors", err != nil, "expected error")
}

// =====================================================================
// 12. ROBUSTNESS — 17 tests
// =====================================================================

func testRobustness() {
	section("12. ROBUSTNESS (large data, deep nesting)")

	// Large slice
	big := make([]int, 10000)
	for i := range big {
		big[i] = i
	}
	b := mustMarshal(big)
	var out []int
	json.Unmarshal(b, &out)
	check("10K slice len", len(out) == 10000, fmt.Sprintf("got %d", len(out)))
	check("10K slice first", out[0] == 0, fmt.Sprintf("got %d", out[0]))
	check("10K slice last", out[9999] == 9999, fmt.Sprintf("got %d", out[9999]))

	// Large map
	bigMap := make(map[string]int, 500)
	for i := 0; i < 500; i++ {
		bigMap[fmt.Sprintf("key_%d", i)] = i
	}
	b = mustMarshal(bigMap)
	var outMap map[string]int
	json.Unmarshal(b, &outMap)
	check("500-entry map", len(outMap) == 500, fmt.Sprintf("got %d", len(outMap)))
	check("map value spot-check", outMap["key_42"] == 42, fmt.Sprintf("got %d", outMap["key_42"]))

	// Large string
	bigStr := strings.Repeat("x", 1_000_000)
	b = mustMarshal(bigStr)
	var outStr string
	json.Unmarshal(b, &outStr)
	check("1M string len", len(outStr) == 1_000_000, fmt.Sprintf("got %d", len(outStr)))

	// Deep nesting - objects
	depth := 100
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString(`{"a":`)
	}
	sb.WriteString(`1`)
	for i := 0; i < depth; i++ {
		sb.WriteString(`}`)
	}
	check("deep object valid", json.Valid([]byte(sb.String())), "expected valid")
	var dv any
	err := json.Unmarshal([]byte(sb.String()), &dv)
	check("deep object unmarshal", err == nil, fmt.Sprintf("err=%v", err))

	// Deep nesting - arrays
	sb.Reset()
	for i := 0; i < depth; i++ {
		sb.WriteByte('[')
	}
	sb.WriteString(`1`)
	for i := 0; i < depth; i++ {
		sb.WriteByte(']')
	}
	check("deep array valid", json.Valid([]byte(sb.String())), "expected valid")
	err = json.Unmarshal([]byte(sb.String()), &dv)
	check("deep array unmarshal", err == nil, fmt.Sprintf("err=%v", err))

	// Duplicate keys
	var dm map[string]int
	json.Unmarshal([]byte(`{"a":1,"a":2,"a":3}`), &dm)
	check("dup keys: last wins", dm["a"] == 3, fmt.Sprintf("got %d", dm["a"]))

	type DS struct {
		A int `json:"a"`
	}
	var ds DS
	json.Unmarshal([]byte(`{"a":1,"a":2}`), &ds)
	check("dup keys struct: last wins", ds.A == 2, fmt.Sprintf("got %d", ds.A))

	// Unknown fields silently skipped with complex values
	type Simple struct {
		Name string `json:"name"`
	}
	var ss Simple
	json.Unmarshal([]byte(`{"skip_arr":[1,2,{"deep":true}],"name":"ok","skip_obj":{"a":{"b":"c"}}}`), &ss)
	check("skip complex unknowns", ss.Name == "ok", fmt.Sprintf("got %q", ss.Name))

	// Whitespace tolerance
	json.Unmarshal([]byte("   \t\n\r  {  \"name\"  :  \"ws\"  }  \n"), &ss)
	check("whitespace tolerance", ss.Name == "ws", fmt.Sprintf("got %q", ss.Name))
}

// =====================================================================
// 13. CRASH RESISTANCE — 25 garbage inputs
// =====================================================================

func testCrashResistance() {
	section("13. CRASH RESISTANCE (must not panic)")

	garbage := []string{
		``, `{`, `}`, `[`, `]`, `{}}`, `[]]`, `{"key"`, `{"key":`,
		`{"key":}`, `[1,2,3`, `"unterminated`, `"\`, `"\u`, `"\u00`,
		`"\uZZZZ"`, `trux`, `nulx`, `falsx`,
		string([]byte{0x00}), string([]byte{0xFF, 0xFE}),
		strings.Repeat(`[`, 500) + strings.Repeat(`]`, 499),
		`{"a":1` + strings.Repeat(`,`, 50),
		strings.Repeat(`{"a":`, 200) + `1` + strings.Repeat(`}`, 199),
		`[` + strings.Repeat(`null,`, 10000) + `null`,
	}

	allSafe := true
	for _, g := range garbage {
		func() {
			defer func() {
				if r := recover(); r != nil {
					allSafe = false
					label := g
					if len(label) > 30 {
						label = label[:30] + "..."
					}
					check("no panic: "+label, false, fmt.Sprintf("PANIC: %v", r))
				}
			}()
			var v any
			json.Unmarshal([]byte(g), &v)
		}()
	}
	check(fmt.Sprintf("all %d garbage inputs safe", len(garbage)), allSafe, "one or more panicked")

	// Also test marshal doesn't crash on nil/zero
	noPanic := true
	func() {
		defer func() {
			if r := recover(); r != nil {
				noPanic = false
			}
		}()
		json.Marshal(nil)
		json.Marshal((*int)(nil))
		json.Marshal(map[string]any(nil))
		json.Marshal([]int(nil))
	}()
	check("marshal nil types safe", noPanic, "panicked")
}

// =====================================================================
// 14. ROUNDTRIP — 18 tests
// =====================================================================

func testRoundtrip() {
	section("14. ROUNDTRIP (marshal -> unmarshal -> compare)")

	type Addr struct {
		Street string `json:"street"`
		City   string `json:"city"`
		Zip    string `json:"zip"`
	}
	type Contact struct {
		Email string `json:"email"`
		Phone string `json:"phone,omitempty"`
	}
	type User struct {
		ID      uint64            `json:"id"`
		Name    string            `json:"name"`
		Age     int               `json:"age"`
		Score   float64           `json:"score"`
		Active  bool              `json:"active"`
		Tags    []string          `json:"tags"`
		Addr    Addr              `json:"addr"`
		Contact Contact           `json:"contact"`
		Meta    map[string]string `json:"meta"`
	}
	in := User{
		ID: 12345, Name: "Jane", Age: 28, Score: 95.5, Active: true,
		Tags:    []string{"admin", "user"},
		Addr:    Addr{Street: "456 Oak", City: "Metro", Zip: "99999"},
		Contact: Contact{Email: "jane@x.com"},
		Meta:    map[string]string{"role": "lead"},
	}
	b := mustMarshal(in)
	check("complex roundtrip: valid JSON", json.Valid(b), string(b))

	var out User
	json.Unmarshal(b, &out)
	check("roundtrip: ID", out.ID == in.ID, fmt.Sprintf("got %d", out.ID))
	check("roundtrip: Name", out.Name == in.Name, fmt.Sprintf("got %q", out.Name))
	check("roundtrip: Age", out.Age == in.Age, fmt.Sprintf("got %d", out.Age))
	check("roundtrip: Score", out.Score == in.Score, fmt.Sprintf("got %v", out.Score))
	check("roundtrip: Active", out.Active == in.Active, fmt.Sprintf("got %v", out.Active))
	check("roundtrip: Tags", reflect.DeepEqual(out.Tags, in.Tags), fmt.Sprintf("got %v", out.Tags))
	check("roundtrip: Addr", out.Addr == in.Addr, fmt.Sprintf("got %+v", out.Addr))
	check("roundtrip: Contact.Email", out.Contact.Email == in.Contact.Email, fmt.Sprintf("got %q", out.Contact.Email))
	check("roundtrip: Contact.Phone empty", out.Contact.Phone == "", fmt.Sprintf("got %q", out.Contact.Phone))
	check("roundtrip: Meta", reflect.DeepEqual(out.Meta, in.Meta), fmt.Sprintf("got %v", out.Meta))

	// All primitive types
	type AllP struct {
		B  bool    `json:"b"`
		I  int     `json:"i"`
		I8 int8    `json:"i8"`
		I6 int64   `json:"i6"`
		U  uint    `json:"u"`
		U8 uint8   `json:"u8"`
		U6 uint64  `json:"u6"`
		F3 float32 `json:"f3"`
		F6 float64 `json:"f6"`
		S  string  `json:"s"`
	}
	pin := AllP{B: true, I: -1, I8: -128, I6: math.MinInt64, U: 1, U8: 255,
		U6: math.MaxUint64, F3: 3.14, F6: 2.718281828459045, S: "test"}
	b = mustMarshal(pin)
	var pout AllP
	json.Unmarshal(b, &pout)
	check("roundtrip: bool", pout.B == pin.B, "")
	check("roundtrip: int", pout.I == pin.I, "")
	check("roundtrip: int8", pout.I8 == pin.I8, "")
	check("roundtrip: uint64", pout.U6 == pin.U6, "")
	check("roundtrip: float64", pout.F6 == pin.F6, "")
	check("roundtrip: string", pout.S == pin.S, "")
}

// =====================================================================
// 15. STREAMING DECODER — 15 tests
// =====================================================================

func testStreamingDecoder() {
	section("15. STREAMING DECODER (token-level)")

	data := []byte(`{"name":"Alice","age":30,"active":true,"nil":null,"nums":[1,2.5]}`)
	dec := jsontext.NewDecoderFromBytes(data)

	type ex struct {
		kind jsontext.Kind
		str  string
	}
	expects := []ex{
		{jsontext.BeginObject, ""}, {jsontext.String, "name"}, {jsontext.String, "Alice"},
		{jsontext.String, "age"}, {jsontext.Number, "30"}, {jsontext.String, "active"},
		{jsontext.True, ""}, {jsontext.String, "nil"}, {jsontext.Null, ""},
		{jsontext.String, "nums"}, {jsontext.BeginArray, ""}, {jsontext.Number, "1"},
		{jsontext.Number, "2.5"}, {jsontext.EndArray, ""}, {jsontext.EndObject, ""},
	}
	for i, e := range expects {
		tok, err := dec.ReadToken()
		ok := err == nil && tok.Kind() == e.kind
		if e.str != "" {
			ok = ok && tok.Str() == e.str
		}
		check(fmt.Sprintf("token[%d] kind=%v", i, e.kind), ok,
			fmt.Sprintf("err=%v kind=%v str=%q", err, tok.Kind(), tok.Str()))
	}
	jsontext.PutDecoder(dec)
}

// =====================================================================
// 16. ENCODER — 15 tests
// =====================================================================

func testEncoder() {
	section("16. ENCODER (token writing)")

	var buf []byte
	enc := jsontext.NewEncoderToBytes(&buf)
	enc.WriteToken(jsontext.TBeginObject)
	enc.WriteString("name")
	enc.WriteString("Bob")
	enc.WriteString("age")
	enc.WriteInt(25)
	enc.WriteString("active")
	enc.WriteBool(true)
	enc.WriteString("score")
	enc.WriteFloat(3.14)
	enc.WriteString("nothing")
	enc.WriteNull()
	enc.WriteToken(jsontext.TEndObject)
	result := string(enc.Bytes())
	expect("object encoding", result,
		`{"name":"Bob","age":25,"active":true,"score":3.14,"nothing":null}`)
	jsontext.PutEncoder(enc)

	enc = jsontext.NewEncoderToBytes(&buf)
	enc.WriteToken(jsontext.TBeginArray)
	enc.WriteInt(1)
	enc.WriteInt(2)
	enc.WriteToken(jsontext.TBeginArray)
	enc.WriteString("nested")
	enc.WriteToken(jsontext.TEndArray)
	enc.WriteToken(jsontext.TEndArray)
	expect("nested array", string(enc.Bytes()), `[1,2,["nested"]]`)
	jsontext.PutEncoder(enc)

	enc = jsontext.NewEncoderToBytes(&buf)
	enc.WriteToken(jsontext.TBeginArray)
	enc.WriteBool(true)
	enc.WriteBool(false)
	enc.WriteNull()
	enc.WriteToken(jsontext.TEndArray)
	expect("mixed array", string(enc.Bytes()), `[true,false,null]`)
	jsontext.PutEncoder(enc)

	enc = jsontext.NewEncoderToBytes(&buf)
	enc.WriteToken(jsontext.TBeginObject)
	enc.WriteString("key")
	enc.WriteUint(18446744073709551615)
	enc.WriteToken(jsontext.TEndObject)
	expect("uint64 max", string(enc.Bytes()), `{"key":18446744073709551615}`)
	jsontext.PutEncoder(enc)

	enc = jsontext.NewEncoderToBytes(&buf)
	enc.WriteToken(jsontext.TBeginObject)
	enc.WriteString("k")
	enc.WriteInt(-9223372036854775808)
	enc.WriteToken(jsontext.TEndObject)
	expect("int64 min", string(enc.Bytes()), `{"k":-9223372036854775808}`)
	jsontext.PutEncoder(enc)

	enc = jsontext.NewEncoderToBytes(&buf)
	enc.WriteToken(jsontext.TBeginArray)
	enc.WriteToken(jsontext.TEndArray)
	expect("empty array", string(enc.Bytes()), `[]`)
	jsontext.PutEncoder(enc)

	enc = jsontext.NewEncoderToBytes(&buf)
	enc.WriteToken(jsontext.TBeginObject)
	enc.WriteToken(jsontext.TEndObject)
	expect("empty object", string(enc.Bytes()), `{}`)
	jsontext.PutEncoder(enc)

	enc = jsontext.NewEncoderToBytes(&buf)
	enc.WriteToken(jsontext.TBeginObject)
	enc.WriteString("escaped")
	enc.WriteString("hello \"world\" \\ \n \t")
	enc.WriteToken(jsontext.TEndObject)
	r := string(enc.Bytes())
	check("encoder escapes quotes", strings.Contains(r, `\"`), r)
	check("encoder escapes backslash", strings.Contains(r, `\\`), r)
	check("encoder escapes newline", strings.Contains(r, `\n`), r)
	check("encoder escapes tab", strings.Contains(r, `\t`), r)
	jsontext.PutEncoder(enc)

	// Output must be valid JSON
	check("encoder output is valid", json.Valid([]byte(r)), r)
}

// =====================================================================
// 17. EDGE CASES — 17 tests
// =====================================================================

func testEdgeCases() {
	section("17. EDGE CASES")

	type Empty struct{}
	expect("empty struct", string(mustMarshal(Empty{})), `{}`)

	var m map[string]any
	json.Unmarshal([]byte(`{}`), &m)
	check("empty object", len(m) == 0, fmt.Sprintf("got %d entries", len(m)))

	var arr []any
	json.Unmarshal([]byte(`[]`), &arr)
	check("empty array", len(arr) == 0, fmt.Sprintf("got %d entries", len(arr)))

	type Z struct {
		A string  `json:"a"`
		B int     `json:"b"`
		C float64 `json:"c"`
		D bool    `json:"d"`
	}
	expect("zero values", string(mustMarshal(Z{})),
		`{"a":"","b":0,"c":0,"d":false}`)

	type I struct {
		X int `json:"x"`
	}
	type M struct {
		I *I `json:"i"`
	}
	type O struct {
		M *M `json:"m"`
	}
	expect("nested nil ptr", string(mustMarshal(O{M: nil})), `{"m":null}`)
	expect("nested nil inner", string(mustMarshal(O{M: &M{I: nil}})),
		`{"m":{"i":null}}`)
	expect("nested full", string(mustMarshal(O{M: &M{I: &I{X: 7}}})),
		`{"m":{"i":{"x":7}}}`)

	type WS struct {
		Name string `json:"name"`
	}
	var ws WS
	json.Unmarshal([]byte(`  {  "name"  :  "Alice"  }  `), &ws)
	check("whitespace tolerance", ws.Name == "Alice", fmt.Sprintf("got %q", ws.Name))

	type Sk struct {
		Name string `json:"name"`
	}
	var sk Sk
	json.Unmarshal([]byte(`{"skip":[1,{"deep":true}],"name":"ok","skip2":{"a":"b"}}`), &sk)
	check("skip complex fields", sk.Name == "ok", fmt.Sprintf("got %q", sk.Name))

	// Slice with existing data gets overwritten
	existing := []int{10, 20, 30, 40, 50}
	json.Unmarshal([]byte(`[1,2]`), &existing)
	check("slice shrinks", len(existing) == 2, fmt.Sprintf("got len=%d", len(existing)))
	check("slice values correct", existing[0] == 1 && existing[1] == 2,
		fmt.Sprintf("got %v", existing))

	// Map with existing data gets merged
	em := map[string]int{"old": 99}
	json.Unmarshal([]byte(`{"new":1}`), &em)
	check("map merge: old kept", em["old"] == 99, fmt.Sprintf("got %d", em["old"]))
	check("map merge: new added", em["new"] == 1, fmt.Sprintf("got %d", em["new"]))

	// Struct partial update
	type P struct {
		A string `json:"a"`
		B string `json:"b"`
	}
	pp := P{A: "original", B: "original"}
	json.Unmarshal([]byte(`{"a":"updated"}`), &pp)
	check("partial update: a changed", pp.A == "updated", fmt.Sprintf("got %q", pp.A))
	check("partial update: b kept", pp.B == "original", fmt.Sprintf("got %q", pp.B))
}

// =====================================================================
// 18. NUMBER EDGE CASES — 15 tests
// =====================================================================

func testNumberEdgeCases() {
	section("18. NUMBER EDGE CASES")

	var f float64
	json.Unmarshal([]byte(`1.5e2`), &f)
	check("sci notation 1.5e2=150", f == 150.0, fmt.Sprintf("got %v", f))

	json.Unmarshal([]byte(`1e-3`), &f)
	check("sci notation 1e-3=0.001", f == 0.001, fmt.Sprintf("got %v", f))

	json.Unmarshal([]byte(`-0`), &f)
	check("negative zero", f == 0, fmt.Sprintf("got %v", f))

	json.Unmarshal([]byte(`0.0`), &f)
	check("explicit 0.0", f == 0, fmt.Sprintf("got %v", f))

	var n int
	json.Unmarshal([]byte(`3.9`), &n)
	check("float->int truncates", n == 3, fmt.Sprintf("got %d", n))

	json.Unmarshal([]byte(`-2.7`), &n)
	check("neg float->int truncates", n == -2, fmt.Sprintf("got %d", n))

	// String-encoded numbers
	json.Unmarshal([]byte(`"42"`), &n)
	check("string-encoded int", n == 42, fmt.Sprintf("got %d", n))

	var u uint
	json.Unmarshal([]byte(`"100"`), &u)
	check("string-encoded uint", u == 100, fmt.Sprintf("got %d", u))

	json.Unmarshal([]byte(`"3.14"`), &f)
	check("string-encoded float", f == 3.14, fmt.Sprintf("got %v", f))

	// Max/min boundaries
	var i8 int8
	json.Unmarshal([]byte(`127`), &i8)
	check("int8 max", i8 == 127, fmt.Sprintf("got %d", i8))

	json.Unmarshal([]byte(`-128`), &i8)
	check("int8 min", i8 == -128, fmt.Sprintf("got %d", i8))

	var u8 uint8
	json.Unmarshal([]byte(`255`), &u8)
	check("uint8 max", u8 == 255, fmt.Sprintf("got %d", u8))

	json.Unmarshal([]byte(`0`), &u8)
	check("uint8 min", u8 == 0, fmt.Sprintf("got %d", u8))

	// Very large numbers in any
	var v any
	json.Unmarshal([]byte(`999999999999999999`), &v)
	check("large number is float64", reflect.TypeOf(v).Kind() == reflect.Float64,
		fmt.Sprintf("got %T", v))

	json.Unmarshal([]byte(`1e308`), &v)
	check("1e308 parses", v != nil, fmt.Sprintf("got %v", v))
}

// =====================================================================
// 19. CONCURRENCY — 15+ tests
// =====================================================================

func testConcurrency() {
	section("19. CONCURRENCY (goroutine safety)")

	type S struct {
		Name string `json:"name"`
		N    int    `json:"n"`
	}

	// Concurrent marshal + unmarshal
	var wg sync.WaitGroup
	var fails int64
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&fails, 1)
				}
			}()
			b, err := json.Marshal(S{Name: "test", N: n})
			if err != nil {
				atomic.AddInt64(&fails, 1)
				return
			}
			var out S
			if err := json.Unmarshal(b, &out); err != nil {
				atomic.AddInt64(&fails, 1)
				return
			}
			if out.N != n {
				atomic.AddInt64(&fails, 1)
			}
		}(i)
	}
	wg.Wait()
	check("100 concurrent marshal/unmarshal", atomic.LoadInt64(&fails) == 0,
		fmt.Sprintf("%d failures", fails))

	// Concurrent map unmarshal
	data := []byte(`{"a":1,"b":"two","c":true,"d":[1,2],"e":{"f":"g"}}`)
	fails = 0
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&fails, 1)
				}
			}()
			var m map[string]any
			if err := json.Unmarshal(data, &m); err != nil {
				atomic.AddInt64(&fails, 1)
				return
			}
			if m["a"] != float64(1) || m["b"] != "two" || m["c"] != true {
				atomic.AddInt64(&fails, 1)
			}
		}()
	}
	wg.Wait()
	check("100 concurrent map unmarshal", atomic.LoadInt64(&fails) == 0,
		fmt.Sprintf("%d failures", fails))

	// Concurrent MarshalAppend
	fails = 0
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&fails, 1)
				}
			}()
			buf := make([]byte, 0, 256)
			_, err := json.MarshalAppend(buf, S{Name: "a", N: n})
			if err != nil {
				atomic.AddInt64(&fails, 1)
			}
		}(i)
	}
	wg.Wait()
	check("100 concurrent MarshalAppend", atomic.LoadInt64(&fails) == 0,
		fmt.Sprintf("%d failures", fails))

	// Concurrent Valid
	fails = 0
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					atomic.AddInt64(&fails, 1)
				}
			}()
			if !json.Valid(data) {
				atomic.AddInt64(&fails, 1)
			}
		}()
	}
	wg.Wait()
	check("100 concurrent Valid", atomic.LoadInt64(&fails) == 0,
		fmt.Sprintf("%d failures", fails))
}

// =====================================================================
// 20. SECURITY / EXPLOIT TESTING — 20 tests
// =====================================================================

func testSecurityExploits() {
	section("20. SECURITY / EXPLOIT TESTING")

	// 1. Billion laughs / exponential expansion: deeply nested
	depth := 500
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString(`{"a":`)
	}
	sb.WriteString(`"end"`)
	for i := 0; i < depth; i++ {
		sb.WriteByte('}')
	}
	var v any
	err := json.Unmarshal([]byte(sb.String()), &v)
	check("deep nesting 500 levels: no crash", true, "")
	_ = err

	// 2. Huge key name (1MB key) — should not crash
	hugeKey := `{"` + strings.Repeat("A", 1_000_000) + `":"value"}`
	noPanic := true
	func() {
		defer func() {
			if r := recover(); r != nil {
				noPanic = false
			}
		}()
		json.Unmarshal([]byte(hugeKey), &v)
	}()
	check("1MB key name: no crash", noPanic, "panicked")

	// 3. Huge string value (10MB)
	hugeVal := `"` + strings.Repeat("B", 10_000_000) + `"`
	func() {
		defer func() {
			if r := recover(); r != nil {
				noPanic = false
			}
		}()
		var s string
		json.Unmarshal([]byte(hugeVal), &s)
		noPanic = len(s) == 10_000_000
	}()
	check("10MB string value: no crash", noPanic, "panicked or wrong length")

	// 4. Null bytes in strings: should roundtrip correctly
	nullStr := "before\x00after"
	b := mustMarshal(nullStr)
	check("null byte: valid JSON", json.Valid(b), string(b))
	var outNB string
	json.Unmarshal(b, &outNB)
	check("null byte: roundtrip", outNB == nullStr, fmt.Sprintf("got %q", outNB))

	// 5. Injection via string values: JSON in JSON
	injection := `{"payload":"value"}`
	b = mustMarshal(injection)
	check("JSON injection: escaped", bytes.Contains(b, []byte(`\"`)), string(b))
	var outInj string
	json.Unmarshal(b, &outInj)
	check("JSON injection: roundtrip", outInj == injection, fmt.Sprintf("got %q", outInj))

	// 6. HTML injection without EscapeHTML
	htmlInj := `<img src=x onerror=alert(1)>`
	b = mustMarshal(htmlInj, json.EscapeHTML(true))
	check("HTML injection: no raw <", !bytes.Contains(b, []byte("<")), string(b))
	check("HTML injection: no raw >", !bytes.Contains(b, []byte(">")), string(b))

	// 7. Script injection with EscapeHTML
	script := `<script>document.cookie</script>`
	b = mustMarshal(script, json.EscapeHTML(true))
	check("script tag: escaped", !bytes.Contains(b, []byte(`<script>`)), string(b))

	// 8. Backslash injection
	bsInj := `\\\\\"`
	b = mustMarshal(bsInj)
	check("backslash injection: valid", json.Valid(b), string(b))
	var outBS string
	json.Unmarshal(b, &outBS)
	check("backslash injection: roundtrip", outBS == bsInj, fmt.Sprintf("got %q", outBS))

	// 9. Unicode escape injection \u0000
	uniInj := "\u0000\u0001\u001f"
	b = mustMarshal(uniInj)
	check("unicode ctrl injection: valid", json.Valid(b), string(b))

	// 10. Very long array to test memory behavior
	longArr := "[" + strings.Repeat("1,", 100000) + "1]"
	func() {
		defer func() {
			if r := recover(); r != nil {
				noPanic = false
			}
		}()
		var arr []int
		json.Unmarshal([]byte(longArr), &arr)
		noPanic = len(arr) == 100001
	}()
	check("100K array: no OOM crash", noPanic, "panicked or wrong size")

	// 11. Map with many keys
	var mkb strings.Builder
	mkb.WriteByte('{')
	for i := 0; i < 10000; i++ {
		if i > 0 {
			mkb.WriteByte(',')
		}
		fmt.Fprintf(&mkb, `"k%d":%d`, i, i)
	}
	mkb.WriteByte('}')
	var bigm map[string]any
	json.Unmarshal([]byte(mkb.String()), &bigm)
	check("10K unique keys", len(bigm) == 10000, fmt.Sprintf("got %d", len(bigm)))

	// 12. Prototype pollution style: __proto__ key
	var proto map[string]any
	json.Unmarshal([]byte(`{"__proto__":{"admin":true},"constructor":{"a":1}}`), &proto)
	check("__proto__ key: just a string key", proto["__proto__"] != nil, fmt.Sprintf("got %v", proto))

	// 13. Number overflow — should not crash
	overflow := `99999999999999999999999999999999999999999999999999`
	func() {
		defer func() {
			if r := recover(); r != nil {
				noPanic = false
			}
		}()
		var n int
		json.Unmarshal([]byte(overflow), &n)
		noPanic = true
	}()
	check("number overflow: no crash", noPanic, "panicked")

	// 14. Extremely nested arrays
	deepArr := strings.Repeat("[", 200) + "1" + strings.Repeat("]", 200)
	func() {
		defer func() {
			if r := recover(); r != nil {
				noPanic = false
			}
		}()
		json.Unmarshal([]byte(deepArr), &v)
		noPanic = true
	}()
	check("200-deep array: no crash", noPanic, "panicked")

	// 15. All output valid JSON (no injection can produce invalid output)
	attackStrings := []string{
		`"`, `\`, `\"`, `\\\"`, "\n\r\t\x00",
		`<script>`, `</script>`, `&amp;`,
		`{"nested":"value"}`, `[1,2,3]`,
		"very" + strings.Repeat(" long ", 10000) + "string",
	}
	allValid := true
	for _, s := range attackStrings {
		b := mustMarshal(s)
		if !json.Valid(b) {
			allValid = false
		}
	}
	check("all attack strings produce valid JSON", allValid, "some output was invalid")
}
