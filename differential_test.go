package json_test

import (
	"math"
	"reflect"
	"strconv"
	"strings"
	"testing"

	alosjson "github.com/guno1928/alos-json"
	"github.com/guno1928/alos-json/jsontext"
)

type DiffEmbeddedPtr struct {
	EP int `json:"ep"`
}

type DiffTarget struct {
	EmbeddedInner
	*DiffEmbeddedPtr
	S      string            `json:"s"`
	I      int               `json:"i"`
	I8     int8              `json:"i8"`
	U8     uint8             `json:"u8"`
	U      uint64            `json:"u"`
	F      float64           `json:"f"`
	F32    float32           `json:"f32"`
	B      bool              `json:"b"`
	P      *int              `json:"p"`
	PS     *string           `json:"ps"`
	SL     []int             `json:"sl"`
	SS     []string          `json:"ss"`
	A      [2]int            `json:"a"`
	M      map[string]int    `json:"m"`
	MK     map[int]string    `json:"mk"`
	MA     map[string]any    `json:"ma"`
	Any    any               `json:"any"`
	Nested *DiffTarget       `json:"nested"`
	L      Level             `json:"l"`
	Raw    jsontext.Value    `json:"raw"`
	Bytes  []byte            `json:"bytes"`
	Q      int               `json:"q,string"`
	MS     map[string]string `json:"ms"`
	SP     []*Small2         `json:"sp"`
}

type Small2 struct {
	X int    `json:"x"`
	Y string `json:"y"`
}

type diffTargetFactory struct {
	name string
	make func() any
}

var diffTargets = []diffTargetFactory{
	{"any", func() any { return new(any) }},
	{"map_any", func() any { return new(map[string]any) }},
	{"struct", func() any { return new(DiffTarget) }},
	{"struct_ptr", func() any { return new(*DiffTarget) }},
	{"small", func() any { return new(Small2) }},
	{"ints", func() any { return new([]int) }},
	{"int8s", func() any { return new([]int8) }},
	{"strings", func() any { return new([]string) }},
	{"floats", func() any { return new([]float64) }},
	{"float32s", func() any { return new([]float32) }},
	{"array3", func() any { return new([3]int) }},
	{"ptr_int", func() any { return new(*int) }},
	{"int8", func() any { return new(int8) }},
	{"uint16", func() any { return new(uint16) }},
	{"float32", func() any { return new(float32) }},
	{"string", func() any { return new(string) }},
	{"bool", func() any { return new(bool) }},
	{"map_int", func() any { return new(map[string]int) }},
	{"map_intkey", func() any { return new(map[int]string) }},
	{"bytes", func() any { return new([]byte) }},
	{"any_slice", func() any { return new([]any) }},
	{"stringer_iface", func() any { return new(interface{ String() string }) }},
	{"smalls", func() any { return new([]Small2) }},
}

var diffOptionSets = [][]alosjson.Option{
	nil,
	{alosjson.MatchCaseInsensitiveNames(true)},
	{alosjson.RejectUnknownMembers(true)},
}

var diffSeeds = []string{
	`{"s":"x","i":1,"i8":-5,"u8":200,"u":18446744073709551615,"f":1.5e3,"f32":0.1,"b":true,"p":3,"ps":"q","sl":[1,2],"ss":["a","b\n"],"a":[1,2],"m":{"k":1},"mk":{"7":"seven"},"ma":{"x":[1,{"y":null}]},"any":[true,false,null,"s",1.25],"nested":{"s":"deep","nested":null},"A":5,"S":"emb","ep":4,"l":9,"raw":{"z":[1]},"bytes":"aGVsbG8=","q":"12","ms":{"a":"b"},"sp":[{"x":1,"y":"z"},null]}`,
	` { "S" : "caps" , "I" : 2 , "unknown" : {"deep":[1,2,{"x":"y"}]} , "s":"dup","s":"dup2" } `,
	`{"s":"esc\"aped\\ \/ \b\f\n\r\t \u00e9 \uD83D\uDE00 \u0041","f":-0,"f32":3.4028235e38,"i":-9223372036854775808,"u8":255}`,
	`{"i":3.9,"i8":"-12","f":"2.5","u":1e3,"sl":[1.5,-2.5e0,0],"a":[0,0],"q":"7"}`,
	`[1,2,3,-4,0,9223372036854775807]`,
	`[1.5,2e-7,-0.0,1E+2,123456789012345678,0.1,1e308,5e-324]`,
	`["a","b\u0000c","","日本語","\ud800",""]`,
	`[{"x":1,"y":"a"},{"y":"b","x":2},{}]`,
	`{"a":1,"b":[true,false,null],"c":{"d":"e"},"7":"seven","-1":"neg"}`,
	`"just a string"`,
	`12345`,
	`-0.5e-3`,
	`true`,
	`null`,
	`"aGVsbG8gd29ybGQ="`,
	`[[1],[2,[3,[4]]]]`,
	`{"nested":{"nested":{"nested":{"s":"x"}}}}`,
	`[300,-129,256,65536]`,
	`{"i8":127,"u8":0,"f32":1e39}`,
	`{"S":"a","EP":1,"ep":2}`,
	`{"raw":  [1, 2 ,3]  ,"l":"x"}`,
	"\t\n\r [ ] \n",
	`{}`,
	`[]`,
}

var diffMutationBytes = []byte(`{}[],:"\ 0-1x.eEn`)

func mutations(seed string) []string {
	out := []string{seed}
	for i := 0; i < len(seed); i++ {
		out = append(out, seed[:i]+seed[i+1:])
		for _, c := range diffMutationBytes {
			if seed[i] != c {
				out = append(out, seed[:i]+string(c)+seed[i+1:])
			}
			out = append(out, seed[:i]+string(c)+seed[i:])
		}
	}
	return out
}

func checkDifferential(t *testing.T, doc string, target diffTargetFactory, opts []alosjson.Option) {
	t.Helper()
	ref, fast := target.make(), target.make()
	refErr := alosjson.UnmarshalReference([]byte(doc), ref, opts...)
	fastErr := alosjson.Unmarshal([]byte(doc), fast, opts...)
	if (refErr == nil) != (fastErr == nil) {
		t.Fatalf("target %s opts %d doc %q: reference err=%v, fast err=%v", target.name, len(opts), doc, refErr, fastErr)
	}
	if refErr == nil && !deepEqualNaN(reflect.ValueOf(ref), reflect.ValueOf(fast)) {
		t.Fatalf("target %s doc %q: values differ\nreference: %#v\nfast:      %#v", target.name, doc, reflect.ValueOf(ref).Elem(), reflect.ValueOf(fast).Elem())
	}
}

func TestUnmarshalDifferentialSeeds(t *testing.T) {
	for _, doc := range append(append([]string{}, diffSeeds...), malformedDocuments...) {
		for _, target := range diffTargets {
			for _, opts := range diffOptionSets {
				checkDifferential(t, doc, target, opts)
			}
		}
	}
}

func TestUnmarshalDifferentialMutations(t *testing.T) {
	cases := 0
	for _, seed := range diffSeeds {
		for _, doc := range mutations(seed) {
			for _, target := range diffTargets {
				checkDifferential(t, doc, target, nil)
				cases++
			}
		}
	}
	t.Logf("checked %d mutated document/target pairs", cases)
}

func TestUnmarshalDifferentialNumbers(t *testing.T) {
	var docs []string
	for _, mag := range []float64{1, 1e-300, 1e-20, 1e-7, 0.1, 1e7, 1e15, 1e16, 1e20, 1e22, 1e23, 1e300} {
		for k := 1; k < 200; k += 7 {
			f := mag * float64(k) / 13
			docs = append(docs, strconv.FormatFloat(f, 'g', -1, 64), strconv.FormatFloat(f, 'e', 16, 64), strconv.FormatFloat(-f, 'f', -1, 64))
			docs = append(docs, strconv.FormatFloat(math.Nextafter(f, 2*f), 'g', -1, 64))
		}
	}
	for _, s := range []string{"9007199254740993", "123456789012345.6", "999999999999999", "9999999999999999", "0.000000000000000000001", "100000000000000000000000", "4.9406564584124654e-324", "2.2250738585072011e-308", "1.7976931348623157e308", "1.7976931348623159e308"} {
		docs = append(docs, s, "-"+s)
	}
	targets := []diffTargetFactory{diffTargets[0], diffTargets[8], diffTargets[9], diffTargets[14], diffTargets[12], diffTargets[13]}
	for _, doc := range docs {
		for _, target := range targets {
			checkDifferential(t, doc, target, nil)
			checkDifferential(t, "["+doc+"]", diffTargets[8], nil)
		}
	}
}

func TestUnmarshalStateReuse(t *testing.T) {
	first := new(DiffTarget)
	if err := alosjson.Unmarshal([]byte(diffSeeds[0]), first); err != nil {
		t.Fatal(err)
	}
	snapshot := new(DiffTarget)
	if err := alosjson.UnmarshalReference([]byte(diffSeeds[0]), snapshot); err != nil {
		t.Fatal(err)
	}
	for i := range 2000 {
		var other any
		doc := `{"key` + strconv.Itoa(i) + `":"` + strings.Repeat("z", i%300) + `","n":[` + strconv.Itoa(i) + `]}`
		if err := alosjson.Unmarshal([]byte(doc), &other); err != nil {
			t.Fatal(err)
		}
	}
	if !deepEqualNaN(reflect.ValueOf(first), reflect.ValueOf(snapshot)) {
		t.Fatalf("earlier decode result changed after later decodes:\n%#v\n%#v", first, snapshot)
	}
}

func FuzzUnmarshalDifferential(f *testing.F) {
	for _, s := range diffSeeds {
		f.Add([]byte(s))
	}
	for _, s := range malformedDocuments {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, target := range diffTargets {
			checkDifferential(t, string(data), target, nil)
		}
	})
}

func deepEqualNaN(a, b reflect.Value) bool {
	if a.IsValid() != b.IsValid() {
		return false
	}
	if !a.IsValid() {
		return true
	}
	if a.Type() != b.Type() {
		return false
	}
	switch a.Kind() {
	case reflect.Float32, reflect.Float64:
		fa, fb := a.Float(), b.Float()
		return fa == fb && math.Signbit(fa) == math.Signbit(fb) || math.IsNaN(fa) && math.IsNaN(fb)
	case reflect.Pointer:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return deepEqualNaN(a.Elem(), b.Elem())
	case reflect.Interface:
		if a.IsNil() || b.IsNil() {
			return a.IsNil() == b.IsNil()
		}
		return deepEqualNaN(a.Elem(), b.Elem())
	case reflect.Slice:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}
		for i := 0; i < a.Len(); i++ {
			if !deepEqualNaN(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Array:
		for i := 0; i < a.Len(); i++ {
			if !deepEqualNaN(a.Index(i), b.Index(i)) {
				return false
			}
		}
		return true
	case reflect.Map:
		if a.IsNil() != b.IsNil() || a.Len() != b.Len() {
			return false
		}
		iter := a.MapRange()
		for iter.Next() {
			bv := b.MapIndex(iter.Key())
			if !bv.IsValid() || !deepEqualNaN(iter.Value(), bv) {
				return false
			}
		}
		return true
	case reflect.Struct:
		for i := 0; i < a.NumField(); i++ {
			if !deepEqualNaN(a.Field(i), b.Field(i)) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(valueInterface(a), valueInterface(b))
	}
}

func valueInterface(v reflect.Value) any {
	switch v.Kind() {
	case reflect.Bool:
		return v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int()
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint()
	case reflect.String:
		return v.String()
	}
	return v.Interface()
}
