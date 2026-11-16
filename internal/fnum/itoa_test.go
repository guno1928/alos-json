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
		for _, floorAtEnd := range []bool{false, true} {
			buf := make([]byte, 64)
			for i := range buf {
				buf[i] = sentinel
			}
			for i := range prefixLen {
				buf[i] = byte('a' + i)
			}
			dst := buf[:prefixLen]
			floor := 0
			if floorAtEnd {
				floor = prefixLen
			}
			got := AppendUint64(dst, floor, v)
			if wantAll := "abcdefghij"[:prefixLen] + want; string(got) != wantAll {
				t.Fatalf("AppendUint64(prefix %d, floor %d, %d) = %q, want %q", prefixLen, floor, v, got, wantAll)
			}
			for i := len(got); i < len(buf); i++ {
				if buf[i] != sentinel {
					t.Fatalf("AppendUint64(%d) wrote past its result at %d", v, i)
				}
			}
		}
	}
	if got := string(AppendInt64(nil, 0, -int64(v))); v <= math.MaxInt64 && got != strconv.FormatInt(-int64(v), 10) {
		t.Fatalf("AppendInt64(%d) = %q", -int64(v), got)
	}
}

func TestAppendUint64MatchesStrconv(t *testing.T) {
	for _, v := range uintEdgeCases() {
		checkAppendUint64(t, v)
	}
	if got := string(AppendInt64(nil, 0, math.MinInt64)); got != strconv.FormatInt(math.MinInt64, 10) {
		t.Fatalf("AppendInt64(MinInt64) = %q", got)
	}
}

func TestDecimalLen(t *testing.T) {
	for _, v := range uintEdgeCases() {
		if got, want := DecimalLen(v), len(strconv.FormatUint(v, 10)); got != want {
			t.Fatalf("DecimalLen(%d) = %d, want %d", v, got, want)
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
