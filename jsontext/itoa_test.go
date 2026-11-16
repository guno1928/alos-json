package jsontext

import (
	"math"
	"strconv"
	"testing"
)

func integerEdgeCases() []uint64 {
	var out []uint64
	for v := uint64(0); v <= 100_000; v++ {
		out = append(out, v)
	}
	for p := uint64(10); p <= 1e19; p *= 10 {
		out = append(out, p-1, p, p+1)
		if p > math.MaxUint64/10 {
			break
		}
	}
	out = append(out, math.MaxUint64, math.MaxUint64-1, math.MaxInt64, math.MaxInt64+1, 1e19-1, 1e19, 1<<63-1)
	for v := uint64(1); v != 0 && v < math.MaxUint64/3; v = v*3 + 7 {
		out = append(out, v)
	}
	return out
}

func TestAppendUint64MatchesStrconv(t *testing.T) {
	prefix := []byte("x:")
	for _, v := range integerEdgeCases() {
		got := appendUint64(append([]byte(nil), prefix...), v)
		if want := "x:" + strconv.FormatUint(v, 10); string(got) != want {
			t.Fatalf("appendUint64(%d) = %q, want %q", v, got, want)
		}
	}
}

func TestAppendInt64MatchesStrconv(t *testing.T) {
	for _, u := range integerEdgeCases() {
		for _, v := range []int64{int64(u), -int64(u), math.MinInt64, math.MaxInt64} {
			if got := string(appendInt64(nil, v)); got != strconv.FormatInt(v, 10) {
				t.Fatalf("appendInt64(%d) = %q", v, got)
			}
		}
	}
}

func FuzzAppendUint64(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(math.MaxUint64))
	f.Fuzz(func(t *testing.T, v uint64) {
		if got := string(appendUint64([]byte("p"), v)); got != "p"+strconv.FormatUint(v, 10) {
			t.Fatalf("appendUint64(%d) = %q", v, got)
		}
	})
}
