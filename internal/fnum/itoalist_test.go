//go:build amd64 && !purego

package fnum

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand/v2"
	"strconv"
	"testing"
)

func referenceInt64List(dst []byte, vs []int64) []byte {
	for _, v := range vs {
		dst = append(strconv.AppendInt(dst, v, 10), ',')
	}
	return dst
}

func referenceUint64List(dst []byte, vs []uint64) []byte {
	for _, v := range vs {
		dst = append(strconv.AppendUint(dst, v, 10), ',')
	}
	return dst
}

func requireListKernel(t testing.TB) {
	t.Helper()
	if !useListKernel {
		t.Skip("integer list kernel needs SSSE3")
	}
}

func edgeUint64s() []uint64 {
	vs := []uint64{0, 1, 2, 9, 10, 11, math.MaxInt64 - 1, math.MaxInt64, math.MaxInt64 + 1, math.MaxUint64 - 1, math.MaxUint64}
	for _, p := range pow10Uint64 {
		vs = append(vs, p-1, p, p+1, p*2-1, p*9, p*9+p-1)
	}
	for k := range 64 {
		p := uint64(1) << k
		vs = append(vs, p-1, p, p+1)
	}
	for _, m := range []uint64{1e8, 1e16} {
		for q := uint64(1); q < 1845 && m*q/q == m; q = q*3 + 1 {
			vs = append(vs, m*q-1, m*q, m*q+1)
		}
	}
	return vs
}

func edgeInt64s() []int64 {
	vs := []int64{math.MinInt64, math.MinInt64 + 1, math.MinInt64 + 2}
	for _, u := range edgeUint64s() {
		v := int64(u)
		vs = append(vs, v, -v)
	}
	return vs
}

func randomInt64s(r *rand.Rand, n int) []int64 {
	vs := make([]int64, n)
	for i := range vs {
		v := int64(r.Uint64() >> r.IntN(64))
		if r.IntN(2) == 0 {
			v = -v
		}
		vs[i] = v
	}
	return vs
}

func checkInt64List(t *testing.T, prefix []byte, vs []int64) {
	t.Helper()
	want := referenceInt64List(append([]byte(nil), prefix...), vs)
	got := AppendIntListKernel(append([]byte(nil), prefix...), vs)
	if !bytes.Equal(got, want) {
		t.Fatalf("AppendIntListKernel(%d values) mismatch\ngot  %q\nwant %q", len(vs), got, want)
	}
	checkListKernels(t, vs, nil)
}

func checkUint64List(t *testing.T, prefix []byte, vs []uint64) {
	t.Helper()
	want := referenceUint64List(append([]byte(nil), prefix...), vs)
	got := AppendUintListKernel(append([]byte(nil), prefix...), vs)
	if !bytes.Equal(got, want) {
		t.Fatalf("AppendUintListKernel(%d values) mismatch\ngot  %q\nwant %q", len(vs), got, want)
	}
	checkListKernels(t, nil, vs)
}

func TestIntegerListEdgeValues(t *testing.T) {
	requireListKernel(t)
	ints := edgeInt64s()
	uints := edgeUint64s()
	checkInt64List(t, nil, ints)
	checkUint64List(t, nil, uints)
	for _, v := range ints {
		checkInt64List(t, []byte("["), []int64{v})
	}
	for _, v := range uints {
		checkUint64List(t, []byte("["), []uint64{v})
	}
}

func TestIntegerListEveryLength(t *testing.T) {
	requireListKernel(t)
	for digits := 1; digits <= 20; digits++ {
		lo := pow10Uint64[digits-1]
		if digits == 1 {
			lo = 0
		}
		hi := uint64(math.MaxUint64)
		if digits < len(pow10Uint64) {
			hi = pow10Uint64[digits] - 1
		}
		uints := []uint64{lo, lo + 1, hi - 1, hi, lo + (hi-lo)/2}
		checkUint64List(t, nil, uints)
		var ints []int64
		for _, u := range uints {
			if u <= math.MaxInt64 {
				ints = append(ints, int64(u), -int64(u))
			}
		}
		checkInt64List(t, nil, ints)
	}
}

func TestIntegerListChunkBoundaries(t *testing.T) {
	requireListKernel(t)
	r := rand.New(rand.NewPCG(1, 2))
	for _, n := range []int{0, 1, 2, 15, 16, 17, listChunkLen - 1, listChunkLen, listChunkLen + 1, 2*listChunkLen - 1, 2 * listChunkLen, 2*listChunkLen + 1, 5*listChunkLen + 3} {
		vs := randomInt64s(r, n)
		checkInt64List(t, nil, vs)
		checkInt64List(t, []byte(`{"values":[`), vs)
		us := make([]uint64, n)
		for i, v := range vs {
			us[i] = uint64(v) >> (i % 64)
		}
		checkUint64List(t, nil, us)
	}
}

func TestIntegerListEmptyAndNil(t *testing.T) {
	requireListKernel(t)
	if got := AppendIntListKernel[int64](nil, nil); len(got) != 0 {
		t.Fatalf("AppendIntListKernel(nil, nil) = %q", got)
	}
	if got := AppendUintListKernel([]byte("x"), []uint64{}); string(got) != "x" {
		t.Fatalf("AppendUintListKernel(x, empty) = %q", got)
	}
}

func TestIntegerListRespectsReservedCapacity(t *testing.T) {
	requireListKernel(t)
	r := rand.New(rand.NewPCG(3, 4))
	for _, n := range []int{1, 7, 64, listChunkLen} {
		for _, vs := range [][]int64{randomInt64s(r, n), make([]int64, n)} {
			const prefixLen = 5
			reserve := n*maxListEntryBytes + listStoreSlack
			backing := bytes.Repeat([]byte{sentinel}, prefixLen+reserve+64)
			dst := backing[: prefixLen : prefixLen+reserve]
			got := AppendIntListKernel(dst, vs)
			if &got[0] != &backing[0] {
				t.Fatalf("n=%d: AppendIntListKernel reallocated despite exact reservation", n)
			}
			for i, c := range backing[prefixLen+reserve:] {
				if c != sentinel {
					t.Fatalf("n=%d: byte %d past capacity overwritten", n, i)
				}
			}
			want := referenceInt64List(append([]byte(nil), backing[:prefixLen]...), vs)
			if !bytes.Equal(got, want) {
				t.Fatalf("n=%d: got %q want %q", n, got, want)
			}
		}
	}
}

func TestIntegerListRandom(t *testing.T) {
	requireListKernel(t)
	r := rand.New(rand.NewPCG(5, 6))
	for range 2000 {
		vs := randomInt64s(r, r.IntN(40))
		checkInt64List(t, nil, vs)
		us := make([]uint64, len(vs))
		for i := range us {
			us[i] = r.Uint64() >> r.IntN(64)
		}
		checkUint64List(t, nil, us)
	}
}

func checkNarrowList[T int | int8 | int16 | int32](t *testing.T, vs []T) {
	t.Helper()
	var want []byte
	for _, v := range vs {
		want = append(strconv.AppendInt(want, int64(v), 10), ',')
	}
	if got := AppendIntListKernel([]byte(nil), vs); !bytes.Equal(got, want) {
		t.Fatalf("AppendIntListKernel[%T](%d values) mismatch\ngot  %q\nwant %q", vs[0], len(vs), got, want)
	}
}

func checkNarrowUintList[T uint | uint16 | uint32](t *testing.T, vs []T) {
	t.Helper()
	var want []byte
	for _, v := range vs {
		want = append(strconv.AppendUint(want, uint64(v), 10), ',')
	}
	if got := AppendUintListKernel([]byte(nil), vs); !bytes.Equal(got, want) {
		t.Fatalf("AppendUintListKernel[%T](%d values) mismatch\ngot  %q\nwant %q", vs[0], len(vs), got, want)
	}
}

func narrowValues[T int | int8 | int16 | int32 | uint | uint16 | uint32](r *rand.Rand, n int, edges ...T) []T {
	vs := make([]T, n)
	for i := range vs {
		if i < len(edges) {
			vs[i] = edges[i]
			continue
		}
		vs[i] = T(r.Uint64() >> r.IntN(64))
	}
	return vs
}

func TestIntegerListNarrowTypes(t *testing.T) {
	requireListKernel(t)
	r := rand.New(rand.NewPCG(7, 8))
	for _, n := range []int{1, 3, 4, 5, wideningChunkLen - 1, wideningChunkLen, wideningChunkLen + 1, listChunkLen + 3, 3*wideningChunkLen + 7} {
		checkNarrowList(t, narrowValues[int8](r, n, math.MinInt8, math.MaxInt8, 0, -1, 9, -10, 99, -100))
		checkNarrowList(t, narrowValues[int16](r, n, math.MinInt16, math.MaxInt16, 0, -1, 9999, -10000))
		checkNarrowList(t, narrowValues[int32](r, n, math.MinInt32, math.MaxInt32, 0, -1, 99999999, -100000000))
		checkNarrowList(t, narrowValues[int](r, n, math.MinInt, math.MaxInt, 0, -1))
		checkNarrowUintList(t, narrowValues[uint16](r, n, math.MaxUint16, 0, 9999, 10000))
		checkNarrowUintList(t, narrowValues[uint32](r, n, math.MaxUint32, 0, 99999999, 100000000))
		checkNarrowUintList(t, narrowValues[uint](r, n, math.MaxUint, 0, 1))
	}
}

func FuzzIntegerList(f *testing.F) {
	seed := make([]byte, 0, 8*len(edgeUint64s()))
	for _, u := range edgeUint64s() {
		seed = binary.LittleEndian.AppendUint64(seed, u)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte{0, 0, 0, 0, 0, 0, 0, 0x80})
	f.Fuzz(func(t *testing.T, data []byte) {
		requireListKernel(t)
		n := len(data) / 8
		ints := make([]int64, n)
		uints := make([]uint64, n)
		for i := range n {
			u := binary.LittleEndian.Uint64(data[i*8:])
			uints[i] = u
			ints[i] = int64(u)
		}
		prefix := data[n*8:]
		checkInt64List(t, prefix, ints)
		checkUint64List(t, prefix, uints)
	})
}
