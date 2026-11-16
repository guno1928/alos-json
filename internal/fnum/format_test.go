package fnum

import (
	"math"
	"math/big"
	"strconv"
	"strings"
	"testing"
)

const formatSampleRounds = 300_000

func powRat(base int64, exp int) *big.Rat {
	p := new(big.Int).Exp(big.NewInt(base), big.NewInt(int64(abs(exp))), nil)
	if exp >= 0 {
		return new(big.Rat).SetInt(p)
	}
	return new(big.Rat).SetFrac(big.NewInt(1), p)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func exactFloorLog(x *big.Rat, base int64, estimate float64) int {
	k := int(math.Floor(estimate)) - 2
	for powRat(base, k+1).Cmp(x) <= 0 {
		k++
	}
	return k
}

func TestLogApproximationsMatchBig(t *testing.T) {
	for q := -1100; q <= 1100; q++ {
		x := powRat(2, q)
		if got, want := floorLog10Pow2(q), exactFloorLog(x, 10, float64(q)*math.Log10(2)); got != want {
			t.Fatalf("floorLog10Pow2(%d) = %d, want %d", q, got, want)
		}
		x34 := new(big.Rat).Mul(x, big.NewRat(3, 4))
		if got, want := floorLog10ThreeQuartersPow2(q), exactFloorLog(x34, 10, float64(q)*math.Log10(2)+math.Log10(0.75)); got != want {
			t.Fatalf("floorLog10ThreeQuartersPow2(%d) = %d, want %d", q, got, want)
		}
	}
	for e := -400; e <= 400; e++ {
		if got, want := floorLog2Pow10Wide(e), exactFloorLog(powRat(10, e), 2, float64(e)*math.Log2(10)); got != want {
			t.Fatalf("floorLog2Pow10Wide(%d) = %d, want %d", e, got, want)
		}
	}
}

func TestSchubfachTableMatchesBig(t *testing.T) {
	if len(schubfachG) != schubfachKMax-schubfachKMin+1 {
		t.Fatalf("table has %d entries, want %d", len(schubfachG), schubfachKMax-schubfachKMin+1)
	}
	mask := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 63), big.NewInt(1))
	for k := schubfachKMin; k <= schubfachKMax; k++ {
		r := floorLog2Pow10Wide(-k) - 125
		v := new(big.Rat).Mul(powRat(10, -k), powRat(2, -r))
		g := new(big.Int).Div(v.Num(), v.Denom())
		g.Add(g, big.NewInt(1))
		got := schubfachG[k-schubfachKMin]
		if got[0] != new(big.Int).Rsh(g, 63).Uint64() || got[1] != new(big.Int).And(g, mask).Uint64() {
			t.Fatalf("schubfachG[%d] = %#x %#x, want %s", k, got[0], got[1], g.Text(16))
		}
	}
}

func strconvShortest(f float64) (string, int) {
	s := strconv.FormatFloat(math.Abs(f), 'e', -1, 64)
	mant, exp, _ := strings.Cut(s, "e")
	e, _ := strconv.Atoi(exp)
	whole, frac, _ := strings.Cut(mant, ".")
	return whole + frac, e - len(frac)
}

func checkShortest64(t *testing.T, f float64) {
	t.Helper()
	digits, exp10, ok := Shortest64(f)
	if !ok {
		return
	}
	if f == 0 {
		if digits != 0 || exp10 != 0 {
			t.Fatalf("Shortest64(%v) = %d e%d, want 0 e0", f, digits, exp10)
		}
		return
	}
	if exp10 < 0 && digits%10 == 0 {
		t.Fatalf("Shortest64(%v) = %de%d has trailing fraction zeros", f, digits, exp10)
	}
	for digits%10 == 0 {
		digits /= 10
		exp10++
	}
	wantDigits, wantExp := strconvShortest(f)
	if got := strconv.FormatUint(digits, 10); got != wantDigits || exp10 != wantExp {
		t.Fatalf("Shortest64(%v) = %se%d, want %se%d", f, got, exp10, wantDigits, wantExp)
	}
}

func shortestEdgeCases() []float64 {
	out := []float64{
		0, math.Copysign(0, -1), 1, -1, 0.1, 0.2, 0.3, 0.1 + 0.2, 1.5, 2, 3, 10, 100, 1e15, 1e16, 1e17, 1e21, 1e22, 1e23,
		math.MaxFloat64, -math.MaxFloat64, math.SmallestNonzeroFloat64, 2.2250738585072014e-308, 2.2250738585072009e-308,
		1 << 53, 1<<53 + 2, 1<<53 - 1, 1 << 63, 5e-324, 9007199254740993, 1.7976931348623157e308, 123456789.125, -72.98765432109877,
		103836841080589.89, 2.98023223876953125e-8, 8.98846567431158e307, 4.35e-310, math.Inf(1), math.NaN(),
	}
	for p := 1e-320; p < 1e308; p *= 10 {
		out = append(out, p, math.Nextafter(p, 0), math.Nextafter(p, math.Inf(1)))
	}
	return out
}

func TestShortest64Edges(t *testing.T) {
	for _, f := range shortestEdgeCases() {
		checkShortest64(t, f)
	}
}

func TestShortest64EveryExponent(t *testing.T) {
	for bq := uint64(1); bq < float64ExpMask; bq++ {
		for _, mant := range []uint64{0, 1, 2, 3, float64MantMask, float64MantMask - 1, float64MantMask / 2, 0x5555555555555 & float64MantMask} {
			checkShortest64(t, math.Float64frombits(bq<<float64MantBits|mant))
		}
	}
}

func TestShortest64MatchesStrconv(t *testing.T) {
	x := uint64(0x9E3779B97F4A7C15)
	for range formatSampleRounds {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		r := x * 2685821657736338717
		checkShortest64(t, math.Float64frombits(r))
		checkShortest64(t, float64(int64(r%2_000_000_000))/1e6)
		checkShortest64(t, float64(r>>11)/float64(uint64(1)<<(r%60)))
		checkShortest64(t, float64(r>>20)*1e-5)
	}
}

func FuzzShortest64(f *testing.F) {
	for _, v := range shortestEdgeCases() {
		f.Add(math.Float64bits(v))
	}
	f.Fuzz(func(t *testing.T, b uint64) {
		checkShortest64(t, math.Float64frombits(b))
	})
}
