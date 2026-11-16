package jsontext

import (
	"math"
	"strconv"
	"testing"
)

const floatSampleRounds = 300_000

func checkFastAppendFloat(t *testing.T, f float64, bits int) {
	t.Helper()
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return
	}
	if bits == 32 {
		f = float64(float32(f))
	}
	got := string(FastAppendFloat([]byte("x:"), f, bits))
	if want := "x:" + encodingJSONFloat(f, bits); got != want {
		t.Fatalf("FastAppendFloat(%v, %d) = %q, want %q", f, bits, got, want)
	}
}

func encodingJSONFloat(f float64, bits int) string {
	abs := math.Abs(f)
	format := byte('f')
	if abs != 0 && (bits == 64 && (abs < 1e-6 || abs >= 1e21) || bits == 32 && (float32(abs) < 1e-6 || float32(abs) >= 1e21)) {
		format = 'e'
	}
	s := strconv.FormatFloat(f, format, -1, bits)
	if n := len(s); format == 'e' && n >= 4 && s[n-4] == 'e' && s[n-3] == '-' && s[n-2] == '0' {
		s = s[:n-2] + s[n-1:]
	}
	return s
}

func floatEdgeCases() []float64 {
	out := []float64{
		0, math.Copysign(0, -1), 1, -1, 0.1, 0.2, 0.3, 0.1 + 0.2, 0.05, 0.005, 0.29, 1.5, 98.5, 87.3, 99.99, 0.01, 0.99,
		1e15, 1e15 + 0.5, 1 << 46, 1<<46 + 0.25, 1 << 53, 1<<53 + 2, 1<<53 - 1, -(1 << 53), 1e18, 1e18 + 128, 1 << 60, 1e21, 1e22, 1e-7,
		103836841080589.89, 34777272633100.582, 494028203031760.25, 12624112454828.951,
		math.MaxFloat64, math.SmallestNonzeroFloat64, math.MaxFloat32, math.SmallestNonzeroFloat32, 16777216, 16777217, 1e7, -1e7,
	}
	for p := 1.0; p < 1e23; p *= 10 {
		out = append(out, p, -p, p+0.5, p-0.25, p/3)
	}
	for _, cut := range []float64{1e-6, 1e21, float64(float32(1e-6)), float64(float32(1e21))} {
		below, above := math.Nextafter(cut, 0), math.Nextafter(cut, math.Inf(1))
		out = append(out, cut, -cut, below, -below, above, -above)
	}
	for p := 1e-5; p > 1e-330; p /= 10 {
		out = append(out, p, -p, p*1.5, p*9.99)
	}
	for p := 1e20; p < math.MaxFloat64/10; p *= 10 {
		out = append(out, p, -p, p*1.5, p*9.99)
	}
	return out
}

func TestFastAppendFloatMatchesEncodingJSON(t *testing.T) {
	for _, f := range floatEdgeCases() {
		checkFastAppendFloat(t, f, 64)
		checkFastAppendFloat(t, f, 32)
	}
	x := uint64(0x9E3779B97F4A7C15)
	for i := 0; i < floatSampleRounds; i++ {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		r := x * 2685821657736338717
		checkFastAppendFloat(t, math.Float64frombits(r), 64)
		checkFastAppendFloat(t, float64(int64(r%2_000_000))/100, 64)
		checkFastAppendFloat(t, float64(int64(r%2_000_000))/10, 64)
		checkFastAppendFloat(t, float64(r>>10)/100, 64)
		checkFastAppendFloat(t, float64(r>>4), 64)
		checkFastAppendFloat(t, float64(math.Float32frombits(uint32(r))), 32)
		checkFastAppendFloat(t, float64(int64(r%200_000))/100, 32)
	}
}

func TestWriteDigitsMatchesStrconv(t *testing.T) {
	for _, v := range integerEdgeCases() {
		want := strconv.FormatUint(v, 10)
		out := make([]byte, len(want))
		writeDigits(out, v)
		if string(out) != want {
			t.Fatalf("writeDigits(%d) = %q, want %q", v, out, want)
		}
	}
}

func FuzzFastAppendFloat(f *testing.F) {
	for _, v := range floatEdgeCases() {
		f.Add(math.Float64bits(v))
	}
	f.Fuzz(func(t *testing.T, bits uint64) {
		v := math.Float64frombits(bits)
		checkFastAppendFloat(t, v, 64)
		checkFastAppendFloat(t, v, 32)
		checkFastAppendFloat(t, float64(int64(bits%10_000_000))/100, 64)
	})
}
