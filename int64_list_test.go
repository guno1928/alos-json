package json_test

import (
	"bytes"
	stdjson "encoding/json"
	"math"
	"math/rand/v2"
	"testing"

	alosjson "github.com/guno1928/alos-json"
)

type namedInt64 int64

type namedUint uint

type int64ListHolder struct {
	Values  []int64      `json:"values"`
	Ints    []int        `json:"ints"`
	Uints   []uint       `json:"uints"`
	Uint64s []uint64     `json:"uint64s"`
	Named   []namedInt64 `json:"named"`
	NamedU  []namedUint  `json:"namedU"`
	I8      []int8       `json:"i8"`
	I16     []int16      `json:"i16"`
	I32     []int32      `json:"i32"`
	U16     []uint16     `json:"u16"`
	U32     []uint32     `json:"u32"`
}

var int64ListLengths = []int{0, 1, 2, 16, 17, 511, 512, 513, 1023, 1024, 1025, 3001}

func makeInt64ListHolder(r *rand.Rand, n int) int64ListHolder {
	h := int64ListHolder{
		Values:  make([]int64, n),
		Ints:    make([]int, n),
		Uints:   make([]uint, n),
		Uint64s: make([]uint64, n),
		Named:   make([]namedInt64, n),
		NamedU:  make([]namedUint, n),
		I8:      make([]int8, n),
		I16:     make([]int16, n),
		I32:     make([]int32, n),
		U16:     make([]uint16, n),
		U32:     make([]uint32, n),
	}
	edges := []uint64{0, 9, 10, 99999999, 100000000, 9999999999999999, 10000000000000000, math.MaxInt64, math.MaxUint64}
	for i := range n {
		u := r.Uint64() >> r.IntN(64)
		if i%7 == 0 {
			u = edges[i%len(edges)]
		}
		v := int64(u)
		if i%3 == 0 {
			v = -v
		}
		if i == n/2 {
			v = math.MinInt64
		}
		h.Values[i], h.Ints[i], h.Uints[i], h.Uint64s[i] = v, int(v), uint(u), u
		h.Named[i], h.NamedU[i] = namedInt64(v), namedUint(u)
		h.I8[i], h.I16[i], h.I32[i] = int8(v), int16(v), int32(v)
		h.U16[i], h.U32[i] = uint16(u), uint32(u)
	}
	return h
}

func TestMarshalInt64ListsMatchStd(t *testing.T) {
	r := rand.New(rand.NewPCG(11, 12))
	for _, n := range int64ListLengths {
		h := makeInt64ListHolder(r, n)
		want, err := stdjson.Marshal(h)
		if err != nil {
			t.Fatal(err)
		}
		got, err := alosjson.Marshal(h)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("n=%d: Marshal mismatch\ngot  %.300s\nwant %.300s", n, got, want)
		}
		prefix := []byte("prefix:")
		got, err = alosjson.MarshalAppend(append([]byte(nil), prefix...), h)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, append(prefix, want...)) {
			t.Fatalf("n=%d: MarshalAppend mismatch", n)
		}
		direct, err := alosjson.Marshal(h.Values)
		if err != nil {
			t.Fatal(err)
		}
		wantDirect, _ := stdjson.Marshal(h.Values)
		if !bytes.Equal(direct, wantDirect) {
			t.Fatalf("n=%d: top-level []int64 mismatch", n)
		}
	}
}

func TestMarshalInt64ListsEmptyAndNil(t *testing.T) {
	type pair struct {
		Nil   []int64 `json:"nil"`
		Empty []int64 `json:"empty"`
		NilU  []uint  `json:"nilU"`
		EmpU  []uint  `json:"empU"`
	}
	v := pair{Empty: []int64{}, EmpU: []uint{}}
	got, err := alosjson.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"nil":[],"empty":[],"nilU":[],"empU":[]}`; string(got) != want {
		t.Fatalf("default nil slices: got %s want %s", got, want)
	}
	got, err = alosjson.Marshal(v, alosjson.FormatNilSliceAsNull(true))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"nil":null,"empty":[],"nilU":null,"empU":[]}`; string(got) != want {
		t.Fatalf("FormatNilSliceAsNull: got %s want %s", got, want)
	}
	got, err = alosjson.Marshal([]int(nil), alosjson.FormatNilSliceAsNull(true))
	if err != nil || string(got) != "null" {
		t.Fatalf("top-level nil []int: got %s, %v", got, err)
	}
}
