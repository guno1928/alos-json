package json

import (
	"errors"
	"math"
	"reflect"
	"testing"
)

type valueZeroer string

func (v valueZeroer) IsZero() bool { return v == "zero" }

type pointerZeroer []int

func (p *pointerZeroer) IsZero() bool { return p != nil && len(*p) == 1 }

type zeroerHolder struct{ N int }

func (z zeroerHolder) IsZero() bool { return z.N < 0 }

func omitCheckSamples() []any {
	var nilInt *int
	one := 1
	var nilErr error
	return []any{
		"", "x",
		[]int(nil), []int{}, []int{1},
		map[string]int(nil), map[string]int{}, map[string]int{"a": 1},
		nilInt, &one,
		any(nil), any(nilInt), any(0), any(""),
		nilErr, errors.New("e"),
		[0]int{}, [2]int{}, [2]int{0, 1},
		0, 7, 0.0, math.Copysign(0, -1), math.NaN(), 1.5,
		false, true,
		struct{ A int }{}, struct{ A int }{1},
		valueZeroer(""), valueZeroer("zero"), valueZeroer("x"),
		pointerZeroer(nil), pointerZeroer{}, pointerZeroer{1}, pointerZeroer{1, 2},
		zeroerHolder{}, zeroerHolder{N: -1},
		(*zeroerHolder)(nil), &zeroerHolder{N: -1},
		(*valueZeroer)(nil),
	}
}

func TestOmitChecksMatchReflect(t *testing.T) {
	for _, sample := range omitCheckSamples() {
		v := reflect.ValueOf(&sample).Elem()
		if sample != nil {
			v = reflect.New(reflect.TypeOf(sample)).Elem()
			v.Set(reflect.ValueOf(sample))
		}
		typ, fp := v.Type(), v.Addr().UnsafePointer()
		if got, want := zeroCheckFor(typ).holds(typ, fp, isZero), isZero(v); got != want {
			t.Errorf("zero check for %v %#v: got %v, want %v", typ, sample, got, want)
		}
		if got, want := emptyCheckFor(typ).holds(typ, fp, isEmpty), isEmpty(v); got != want {
			t.Errorf("empty check for %v %#v: got %v, want %v", typ, sample, got, want)
		}
	}
}

type omitTagged struct {
	S    string         `json:"s,omitempty"`
	SZ   string         `json:"sz,omitzero"`
	L    []int          `json:"l,omitempty"`
	LZ   []int          `json:"lz,omitzero"`
	M    map[string]int `json:"m,omitempty"`
	MZ   map[string]int `json:"mz,omitzero"`
	P    *int           `json:"p,omitempty"`
	PZ   *int           `json:"pz,omitzero"`
	I    any            `json:"i,omitempty"`
	IZ   any            `json:"iz,omitzero"`
	V    valueZeroer    `json:"v,omitzero"`
	PV   pointerZeroer  `json:"pv,omitzero"`
	H    zeroerHolder   `json:"h,omitzero"`
	HP   *zeroerHolder  `json:"hp,omitzero"`
	F    float64        `json:"f,omitzero"`
	FE   float64        `json:"fe,omitempty"`
	Arr  [1]int         `json:"arr,omitzero"`
	Both []int          `json:"both,omitempty,omitzero"`
}

func TestOmitTaggedFieldsMarshal(t *testing.T) {
	var nilInt *int
	cases := []struct {
		in   omitTagged
		want string
	}{
		{omitTagged{}, `{"v":"","pv":[],"h":{"N":0}}`},
		{omitTagged{L: []int{}, LZ: []int{}, M: map[string]int{}, MZ: map[string]int{}, Both: []int{}}, `{"lz":[],"mz":{},"v":"","pv":[],"h":{"N":0}}`},
		{omitTagged{I: nilInt, IZ: nilInt, V: "zero", PV: pointerZeroer{1}, H: zeroerHolder{N: -1}, HP: &zeroerHolder{N: -1}}, `{"i":null,"iz":null,"hp":{"N":-1}}`},
		{omitTagged{S: "a", SZ: "b", F: math.Copysign(0, -1), FE: math.Copysign(0, -1)}, `{"s":"a","sz":"b","v":"","pv":[],"h":{"N":0}}`},
	}
	for i, c := range cases {
		got, err := Marshal(&c.in)
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if string(got) != c.want {
			t.Errorf("case %d:\ngot  %s\nwant %s", i, got, c.want)
		}
	}
}
