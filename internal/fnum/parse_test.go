package fnum

import (
	"math"
	"math/big"
	"strconv"
	"testing"
)

const parseSampleRounds = 200_000

func TestPow10TableMatchesBigRational(t *testing.T) {
	one, ten := big.NewInt(1), big.NewInt(10)
	b1p64 := new(big.Int).Lsh(one, 64)
	r1p128 := new(big.Rat).SetInt(new(big.Int).Lsh(one, 128))
	two := big.NewRat(2, 1)
	if len(pow10Table) != pow10Max-pow10Min+1 {
		t.Fatalf("table has %d entries, want %d", len(pow10Table), pow10Max-pow10Min+1)
	}
	for e := pow10Min; e <= pow10Max; e++ {
		var r *big.Rat
		if e >= 0 {
			r = new(big.Rat).SetInt(new(big.Int).Exp(ten, big.NewInt(int64(e)), nil))
		} else {
			r = new(big.Rat).SetFrac(one, new(big.Int).Exp(ten, big.NewInt(int64(-e)), nil))
		}
		shift := 0
		for r.Cmp(r1p128) < 0 {
			r.Mul(r, two)
			shift++
		}
		for r.Cmp(r1p128) >= 0 {
			r.Quo(r, two)
			shift--
		}
		d := new(big.Int).Div(r.Num(), r.Denom())
		hi, lo := new(big.Int).DivMod(d, b1p64, new(big.Int))
		got := pow10Table[e-pow10Min]
		if got[0] != hi.Uint64() || got[1] != lo.Uint64() {
			t.Fatalf("pow10Table[1e%d] = %#x %#x, want %#x %#x", e, got[0], got[1], hi.Uint64(), lo.Uint64())
		}
		if want := 128 - shift; 1+floorLog2Pow10(e) != want {
			t.Fatalf("1+floorLog2Pow10(%d) = %d, want %d", e, 1+floorLog2Pow10(e), want)
		}
	}
}

func checkParseFloat64(t *testing.T, s string) bool {
	t.Helper()
	got, ok := ParseFloat64(s)
	want, err := strconv.ParseFloat(s, 64)
	if !ok {
		return false
	}
	if err != nil {
		t.Fatalf("ParseFloat64(%q) = %v, ok; strconv rejects it: %v", s, got, err)
	}
	if math.Float64bits(got) != math.Float64bits(want) {
		t.Fatalf("ParseFloat64(%q) = %v (%#x), want %v (%#x)", s, got, math.Float64bits(got), want, math.Float64bits(want))
	}
	return true
}

func parseEdgeCases() []string {
	return []string{
		"0", "-0", "0.0", "-0.0", "0e0", "-0e-5", "0e400", "-0e400", "1", "-1", "0.1", "0.2", "0.3", "0.30000000000000004",
		"9007199254740992", "9007199254740993", "9007199254740991", "18446744073709551615", "9999999999999999999", "10000000000000000000",
		"1e22", "1e23", "1e-22", "1e-23", "123456789012345678e-30", "1.7976931348623157e308", "1.7976931348623159e308", "1e308", "1e309",
		"2.2250738585072014e-308", "2.2250738585072011e-308", "4.9e-324", "5e-324", "2e-324", "3e-324", "1e-400", "1e-350", "1e-348", "1e347",
		"-72.98765432109877", "-72.987654321098765", "103836841080589.89", "0.000000000000000000001", "1E5", "1e+5", "1E-5",
		"", "-", "1e", "1e+", "1.e5", ".5", "1.", "1x", "1e5x", "--1", "1e1000", "1e0001", "00012", "7.2057594037927933e16",
		"2.98023223876953125e-8", "8.98846567431158e307", "4.940656458412465441765687928682213723651e-324",
	}
}

func TestParseFloat64Edges(t *testing.T) {
	for _, s := range parseEdgeCases() {
		checkParseFloat64(t, s)
	}
}

func TestParseFloat64MatchesStrconv(t *testing.T) {
	x := uint64(0x9E3779B97F4A7C15)
	next := func() uint64 {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		return x * 2685821657736338717
	}
	accepted, total := 0, 0
	try := func(s string) {
		total++
		if checkParseFloat64(t, s) {
			accepted++
		}
	}
	for range parseSampleRounds {
		r := next()
		f := math.Float64frombits(r)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		try(strconv.FormatFloat(f, 'g', -1, 64))
		try(strconv.FormatFloat(f, 'e', 16, 64))
		try(strconv.FormatFloat(f, 'e', 18, 64))
		try(strconv.FormatFloat(f, 'e', int(r%20), 64))
		small := float64(int64(next()%2_000_000_000)) / 1e6
		try(strconv.FormatFloat(small, 'f', -1, 64))
		try(strconv.FormatFloat(small, 'f', 17, 64))
		mant := next() % 10_000_000_000_000_000_000
		exp := int(next()%700) - 350
		try(strconv.FormatUint(mant, 10) + "e" + strconv.Itoa(exp))
		try("-" + strconv.FormatUint(mant>>uint(r%60), 10) + "e" + strconv.Itoa(exp/16))
	}
	if accepted*4 < total*3 {
		t.Fatalf("fast path accepted only %d/%d inputs", accepted, total)
	}
	t.Logf("fast path accepted %d/%d inputs", accepted, total)
}

func TestEightDigitSWARMatchesScalar(t *testing.T) {
	x := uint64(0x243F6A8885A308D3)
	for range parseSampleRounds {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		r := x * 2685821657736338717
		var b [8]byte
		for j := range b {
			b[j] = byte('0' + (r>>(j*5))%10)
		}
		if r%3 == 0 {
			b[r%8] = byte(r >> 40)
		}
		s := string(b[:])
		want, allDigits := uint64(0), true
		for j := range b {
			if b[j] < '0' || b[j] > '9' {
				allDigits = false
			}
			want = want*10 + uint64(b[j]-'0')
		}
		v := load8(s, 0)
		if isEightDigits(v) != allDigits {
			t.Fatalf("isEightDigits(%q) = %v, want %v", s, !allDigits, allDigits)
		}
		if allDigits && parseEightDigits(v) != want {
			t.Fatalf("parseEightDigits(%q) = %d, want %d", s, parseEightDigits(v), want)
		}
	}
	for c := 0; c < 256; c++ {
		for pos := range 8 {
			b := []byte("12345678")
			b[pos] = byte(c)
			if got, want := isEightDigits(load8(string(b), 0)), c >= '0' && c <= '9'; got != want {
				t.Fatalf("isEightDigits(%q) = %v, want %v", b, got, want)
			}
		}
	}
}

func FuzzParseFloat64(f *testing.F) {
	for _, s := range parseEdgeCases() {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		checkParseFloat64(t, s)
	})
}
