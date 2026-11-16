package fnum

import (
	"bytes"
	"encoding/binary"
	"math"
	"strconv"
	"testing"
)

const sentinel = 0xA5

func TestEightDigitWordExhaustive(t *testing.T) {
	counter := []byte("00000000")
	for n := range uint32(eightDigitsMod) {
		var got [wordBytes]byte
		binary.LittleEndian.PutUint64(got[:], eightDigitWord(n))
		if !bytes.Equal(got[:], counter) {
			t.Fatalf("eightDigitWord(%d) = %q, want %q", n, got, counter)
		}
		for i := len(counter) - 1; i >= 0; i-- {
			if counter[i] < '9' {
				counter[i]++
				break
			}
			counter[i] = '0'
		}
	}
}

func TestPutLeadExhaustive(t *testing.T) {
	var buf [wordBytes]byte
	for n := range uint32(eightDigitsMod) {
		k := DecimalLen(uint64(n))
		putLead(buf[:k], n)
		if k > 1 && buf[0] == '0' {
			t.Fatalf("putLead(%d) = %q has a leading zero", n, buf[:k])
		}
		back := uint32(0)
		for _, c := range buf[:k] {
			if c < '0' || c > '9' {
				t.Fatalf("putLead(%d) = %q", n, buf[:k])
			}
			back = back*10 + uint32(c-'0')
		}
		if back != n {
			t.Fatalf("putLead(%d) = %q", n, buf[:k])
		}
	}
}

func uintEdgeCases() []uint64 {
	out := []uint64{0, 1, 9, 10, 99, 100, 101, math.MaxUint64, math.MaxUint64 - 1, math.MaxInt64, 1 << 63}
	for p := uint64(1); p <= 1e19; p *= 10 {
		out = append(out, p-1, p, p+1, p*2, p*9)
		if p > math.MaxUint64/10 {
			break
		}
	}
	for v := uint64(1); v < math.MaxUint64/7; v = v*7 + 3 {
		out = append(out, v)
	}
	return out
}

func checkAppendUint64(t *testing.T, v uint64) {
	t.Helper()
	want := strconv.FormatUint(v, 10)
	for prefixLen := range 10 {
		buf := make([]byte, 64)
		for i := range buf {
			buf[i] = sentinel
		}
		for i := range prefixLen {
			buf[i] = byte('a' + i)
		}
		got := AppendUint64(buf[:prefixLen], v)
		if wantAll := "abcdefghij"[:prefixLen] + want; string(got) != wantAll {
			t.Fatalf("AppendUint64(prefix %d, %d) = %q, want %q", prefixLen, v, got, wantAll)
		}
		for i := len(got); i < len(buf); i++ {
			if buf[i] != sentinel {
				t.Fatalf("AppendUint64(%d) wrote past its result at %d", v, i)
			}
		}
	}
	if got := string(AppendInt64(nil, -int64(v))); v <= math.MaxInt64 && got != strconv.FormatInt(-int64(v), 10) {
		t.Fatalf("AppendInt64(%d) = %q", -int64(v), got)
	}
}

func TestAppendUint64MatchesStrconv(t *testing.T) {
	for _, v := range uintEdgeCases() {
		checkAppendUint64(t, v)
	}
	if got := string(AppendInt64(nil, math.MinInt64)); got != strconv.FormatInt(math.MinInt64, 10) {
		t.Fatalf("AppendInt64(MinInt64) = %q", got)
	}
}

func TestPutDecimalMatchesStrconv(t *testing.T) {
	for _, v := range uintEdgeCases() {
		want := strconv.FormatUint(v, 10)
		out := make([]byte, len(want))
		PutDecimal(out, v)
		if string(out) != want {
			t.Fatalf("PutDecimal(%d) = %q, want %q", v, out, want)
		}
	}
}

func TestDecimalLen(t *testing.T) {
	for _, v := range uintEdgeCases() {
		if got, want := DecimalLen(v), len(strconv.FormatUint(v, 10)); got != want {
			t.Fatalf("DecimalLen(%d) = %d, want %d", v, got, want)
		}
	}
}

const intRandomSamples = 10_000_000

func TestAppendIntRandomMatchesStrconv(t *testing.T) {
	x := uint64(0xB7E151628AED2A6B)
	var buf []byte
	for i := range intRandomSamples {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		r := x * 2685821657736338717
		v := r >> (r % 64)
		buf = AppendUint64(buf[:0], v)
		if string(buf) != strconv.FormatUint(v, 10) {
			t.Fatalf("AppendUint64(%d) = %q", v, buf)
		}
		s := int64(v)
		if i&1 != 0 {
			s = -s
		}
		buf = AppendInt64(buf[:0], s)
		if string(buf) != strconv.FormatInt(s, 10) {
			t.Fatalf("AppendInt64(%d) = %q", s, buf)
		}
	}
}

func FuzzAppendUint64(f *testing.F) {
	for _, v := range uintEdgeCases() {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, v uint64) {
		checkAppendUint64(t, v)
	})
}
