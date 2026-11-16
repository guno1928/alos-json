package fnum

import (
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/guno1928/alos-json/internal/scan"
)

const scanSampleRounds = 1_500_000

var scanSuffixes = []string{"", ",", "]", "}", " ", "x", "e", "E5", ".", "-", "+1", "0123456789", ",1.5,2.5,3.5"}

func referenceScan(b []byte, i int) (float64, int, bool) {
	end, ok := scan.NumberEnd(b, i)
	if !ok {
		return 0, end, false
	}
	text := string(b[i:end])
	if f, ok := ParseFloat64(text); ok {
		return f, end, true
	}
	f, err := strconv.ParseFloat(text, 64)
	return f, end, err == nil
}

var scanPadding = strings.Repeat(" ", scanWindow)

func checkScanFloat64(t *testing.T, prefix, number, suffix string) bool {
	t.Helper()
	padded := checkScanBuffer(t, []byte(prefix+number+suffix+scanPadding), len(prefix))
	return checkScanBuffer(t, []byte(prefix+number+suffix), len(prefix)) || padded
}

func checkScanBuffer(t *testing.T, b []byte, i int) bool {
	t.Helper()
	got, end, ok := ScanFloat64(b, i)
	slow, slowEnd, slowOK := scanFloat64Slow(b, i)
	if ok != slowOK || ok && (end != slowEnd || math.Float64bits(got) != math.Float64bits(slow)) {
		t.Fatalf("ScanFloat64(%q) = %v, %d, %v; scanFloat64Slow = %v, %d, %v", b, got, end, ok, slow, slowEnd, slowOK)
	}
	if !ok {
		return false
	}
	want, wantEnd, wantOK := referenceScan(b, i)
	if !wantOK {
		t.Fatalf("ScanFloat64(%q) = %v, %d, ok; reference rejects it", b, got, end)
	}
	if end != wantEnd || math.Float64bits(got) != math.Float64bits(want) {
		t.Fatalf("ScanFloat64(%q) = %v (%#x) end %d, want %v (%#x) end %d", b, got, math.Float64bits(got), end, want, math.Float64bits(want), wantEnd)
	}
	return true
}

func checkScanAllSuffixes(t *testing.T, number string) int {
	t.Helper()
	accepted := 0
	for _, suffix := range scanSuffixes {
		for _, prefix := range []string{"", "[", "  :"} {
			if checkScanFloat64(t, prefix, number, suffix) {
				accepted++
			}
		}
	}
	return accepted
}

func TestScanFloat64Edges(t *testing.T) {
	for _, s := range parseEdgeCases() {
		checkScanAllSuffixes(t, s)
	}
	for _, s := range []string{"0", "-0", "01", "-01", "00.5", "0.", "-.5", "0e", "0e+", "0e-1", "1e999", "1e0999", "12345678", "123456789", "1234567890123456789", "12345678901234567890", "0.12345678", "0.123456789", "1.0000000000000000001"} {
		checkScanAllSuffixes(t, s)
	}
}

func TestScanFloat64EveryTerminatorByte(t *testing.T) {
	for _, number := range []string{"1", "-12", "0.5", "123.45678", "-72.98765432109877", "1e5", "1.5e-7", "12345678.123456789"} {
		for c := 0; c < 256; c++ {
			for pad := range 10 {
				suffix := string([]byte{byte(c)}) + "0000000000"[:pad]
				checkScanFloat64(t, "", number, suffix)
			}
		}
	}
}

func TestScanFloat64MatchesReference(t *testing.T) {
	x := uint64(0x6A09E667F3BCC909)
	next := func() uint64 {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		return x * 2685821657736338717
	}
	accepted, total := 0, 0
	try := func(s string) {
		total++
		suffix := scanSuffixes[next()%uint64(len(scanSuffixes))]
		if checkScanFloat64(t, "", s, suffix) {
			accepted++
		}
	}
	for range scanSampleRounds {
		r := next()
		f := math.Float64frombits(r)
		if math.IsNaN(f) || math.IsInf(f, 0) {
			continue
		}
		try(strconv.FormatFloat(f, 'g', -1, 64))
		try(strconv.FormatFloat(f, 'e', int(r%20), 64))
		small := float64(int64(next()%2_000_000_000)) / 1e6
		try(strconv.FormatFloat(small, 'f', -1, 64))
		try(strconv.FormatFloat(small, 'f', int(r%18), 64))
		mant := next() % 10_000_000_000_000_000_000
		exp := int(next()%700) - 350
		try(strconv.FormatUint(mant, 10) + "e" + strconv.Itoa(exp))
		try("-" + strconv.FormatUint(mant>>uint(r%60), 10) + "." + strconv.FormatUint(next()>>uint(r%64), 10))
		try(strconv.FormatInt(int64(next()), 10))
	}
	if accepted*4 < total*3 {
		t.Fatalf("fast path accepted only %d/%d inputs", accepted, total)
	}
	t.Logf("fast path accepted %d/%d inputs", accepted, total)
}

func FuzzScanFloat64(f *testing.F) {
	for _, s := range parseEdgeCases() {
		f.Add(s, ",")
	}
	f.Fuzz(func(t *testing.T, number, suffix string) {
		checkScanFloat64(t, "", number, suffix)
	})
}
