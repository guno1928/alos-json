package fnum

import (
	"encoding/json"
	"math"
	"strconv"
	"testing"
)

const (
	putFloatRandomSamples = 10_000_000
	putFloatGarbage       = 0xA5
)

func encodingJSONFloat64(f float64) string {
	abs := math.Abs(f)
	format := byte('f')
	if abs != 0 && (abs < 1e-6 || abs >= 1e21) {
		format = 'e'
	}
	s := strconv.FormatFloat(f, format, -1, 64)
	if n := len(s); format == 'e' && n >= 4 && s[n-4] == 'e' && s[n-3] == '-' && s[n-2] == '0' {
		s = s[:n-2] + s[n-1:]
	}
	return s
}

func checkPutFloat64(t *testing.T, f float64) {
	t.Helper()
	var w [FloatWindow]byte
	for i := range w {
		w[i] = putFloatGarbage
	}
	n := PutFloat64(&w, f)
	if math.IsNaN(f) || math.IsInf(f, 0) {
		if n != 0 {
			t.Fatalf("PutFloat64(%v) = %d, want 0", f, n)
		}
		return
	}
	if n == 0 {
		if b := math.Float64bits(f) &^ float64SignBit; b>>float64MantBits != 0 {
			t.Fatalf("PutFloat64(%v) declined a normal float", f)
		}
		return
	}
	if got, want := string(w[:n]), encodingJSONFloat64(f); got != want {
		t.Fatalf("PutFloat64(%v) = %q, want %q", f, got, want)
	}
}

func putFloatEdgeCases() []float64 {
	out := append([]float64(nil), shortestEdgeCases()...)
	out = append(out, 0.05, 0.005, 0.29, 98.5, 87.3, 99.99, 0.01, 0.99, 1e15+0.5, 1<<46+0.25, 1e18+128, 1<<60, 1e-7,
		34777272633100.582, 494028203031760.25, 12624112454828.951, 1e7, 1234567.125, 12345678.5, 123456789012345.6,
		1234567890123456.8, 0.000123456789, 0.1234567890123456, 1.0000000000000002, 9.999999999999998, 4.940656458412465e-324)
	for p := 1.0; p < 1e23; p *= 10 {
		out = append(out, p, p+0.5, p-0.25, p/3, p*1.5, p*9.99)
	}
	for _, cut := range []float64{1e-6, 1e-5, 1e16, 1e17, 1e21} {
		out = append(out, cut, math.Nextafter(cut, 0), math.Nextafter(cut, math.Inf(1)))
	}
	for p := 1e-5; p > 1e-330; p /= 10 {
		out = append(out, p, p*1.5, p*9.99, p*1.2345678901234567)
	}
	for p := 1e20; p < math.MaxFloat64/10; p *= 10 {
		out = append(out, p, p*1.5, p*9.99, p*1.2345678901234567)
	}
	n := len(out)
	for i := range n {
		out = append(out, -out[i])
	}
	return out
}

func TestPutFloat64Edges(t *testing.T) {
	for _, f := range putFloatEdgeCases() {
		checkPutFloat64(t, f)
	}
}

func TestPutFloat64ReferenceIsEncodingJSON(t *testing.T) {
	for _, f := range putFloatEdgeCases() {
		if math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		want, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		if got := encodingJSONFloat64(f); got != string(want) {
			t.Fatalf("reference(%v) = %q, encoding/json = %q", f, got, want)
		}
	}
}

func TestPutFloat64EveryExponent(t *testing.T) {
	mants := []uint64{0, 1, 2, 3, float64MantMask, float64MantMask - 1, float64MantMask / 2, 0x5555555555555 & float64MantMask, 0x3333333333333}
	for bq := uint64(0); bq <= float64ExpMask; bq++ {
		for _, mant := range mants {
			b := bq<<float64MantBits | mant
			checkPutFloat64(t, math.Float64frombits(b))
			checkPutFloat64(t, math.Float64frombits(b|float64SignBit))
		}
	}
}

func TestPutFloat64RandomMatchesEncodingJSON(t *testing.T) {
	x := uint64(0x2545F4914F6CDD1D)
	for i := range putFloatRandomSamples {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		r := x * 2685821657736338717
		switch i % 6 {
		case 0:
			checkPutFloat64(t, math.Float64frombits(r))
		case 1:
			checkPutFloat64(t, float64(int64(r%2_000_000_000))/1e6-1000)
		case 2:
			checkPutFloat64(t, -73.0+float64(r%997)*0.0123456789+float64(r%13)/1e7)
		case 3:
			checkPutFloat64(t, float64(int64(r))/math.Pow10(int(r%40)))
		case 4:
			checkPutFloat64(t, float64(r>>11)*math.Pow10(int(r%30)-24))
		default:
			checkPutFloat64(t, float64(int64(r%2_000_000))/math.Pow10(int(r%9)))
		}
	}
}

func FuzzPutFloat64(f *testing.F) {
	for _, v := range putFloatEdgeCases() {
		f.Add(math.Float64bits(v))
	}
	f.Fuzz(func(t *testing.T, b uint64) {
		checkPutFloat64(t, math.Float64frombits(b))
		checkPutFloat64(t, float64(int64(b%10_000_000))/100)
		checkPutFloat64(t, float64(int64(b))/math.Pow10(int(b%23)))
	})
}
