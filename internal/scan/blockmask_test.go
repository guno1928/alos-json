package scan

import "testing"

func specialMaskReference(b []byte) uint64 {
	var m uint64
	for i := range BlockLen {
		if c := b[i]; c < 0x20 || c == '"' || c == '\\' {
			m |= 1 << i
		}
	}
	return m
}

func TestSpecialMask64MatchesReference(t *testing.T) {
	buf := make([]byte, BlockLen+7)
	x := uint64(0x9E3779B97F4A7C15)
	check := func(b []byte) {
		t.Helper()
		want := specialMaskReference(b)
		if got := SpecialMask64(b); got != want {
			t.Fatalf("SpecialMask64(%q) = %064b, want %064b", b, got, want)
		}
		if got := specialMask64Generic(b); got != want {
			t.Fatalf("specialMask64Generic(%q) = %064b, want %064b", b, got, want)
		}
	}
	for c := range 256 {
		for pos := range BlockLen {
			for off := range 8 {
				b := buf[off : off+BlockLen]
				for i := range b {
					b[i] = 'a'
				}
				b[pos] = byte(c)
				check(b)
			}
		}
	}
	for range 200_000 {
		x ^= x >> 12
		x ^= x << 25
		x ^= x >> 27
		r := x * 2685821657736338717
		b := buf[r%8 : r%8+BlockLen]
		for i := range b {
			x ^= x >> 12
			x ^= x << 25
			x ^= x >> 27
			b[i] = byte(x >> 56)
			if x&3 != 0 {
				b[i] = 'a' + byte(x%26)
			}
		}
		check(b)
	}
}
