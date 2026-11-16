package scan

import "testing"

func needsEscapeReference(c byte) bool {
	return c < 0x20 || c == '"' || c == '\\'
}

func needsEscapeHTMLReference(c byte) bool {
	return needsEscapeReference(c) || c == '<' || c == '>' || c == '&'
}

func firstIndexReference(b []byte, match func(byte) bool) int {
	for i, c := range b {
		if match(c) {
			return i
		}
	}
	return len(b)
}

func isBackslash(c byte) bool { return c == '\\' }

const (
	kernelMaxLen     = 80
	kernelMaxOffset  = 16
	kernelFillerByte = 'a'
)

func forEachKernelInput(t *testing.T, check func(t *testing.T, b []byte)) {
	backing := make([]byte, kernelMaxOffset+kernelMaxLen+64)
	for offset := range kernelMaxOffset {
		for n := 0; n <= kernelMaxLen; n++ {
			b := backing[offset : offset+n]
			for i := range b {
				b[i] = kernelFillerByte
			}
			check(t, b)
			for pos := range n {
				for c := range 256 {
					b[pos] = byte(c)
					check(t, b)
				}
				b[pos] = kernelFillerByte
			}
		}
	}
}

func TestFindSafeStringKernelMatchesReference(t *testing.T) {
	forEachKernelInput(t, func(t *testing.T, b []byte) {
		if len(b) == 0 {
			return
		}
		got := findSafeStringSIMD(&b[0], len(b))
		if want := firstIndexReference(b, needsEscapeReference); got != want {
			t.Fatalf("findSafeStringSIMD(%q) = %d, want %d", b, got, want)
		}
	})
}

func TestFindSafeStringHTMLKernelMatchesReference(t *testing.T) {
	forEachKernelInput(t, func(t *testing.T, b []byte) {
		if len(b) == 0 {
			return
		}
		got := findSafeStringHTMLSIMD(&b[0], len(b))
		if want := firstIndexReference(b, needsEscapeHTMLReference); got != want {
			t.Fatalf("findSafeStringHTMLSIMD(%q) = %d, want %d", b, got, want)
		}
	})
}

func TestCountBackslashKernelMatchesReference(t *testing.T) {
	forEachKernelInput(t, func(t *testing.T, b []byte) {
		if len(b) == 0 {
			return
		}
		got := countBackslashSIMD(&b[0], len(b))
		if want := firstIndexReference(b, isBackslash); got != want {
			t.Fatalf("countBackslashSIMD(%q) = %d, want %d", b, got, want)
		}
	})
}

func TestQuoteCopyKernelMatchesReference(t *testing.T) {
	dst := make([]byte, 0, kernelMaxLen+kernelMaxOffset)
	forEachKernelInput(t, func(t *testing.T, b []byte) {
		want := firstIndexReference(b, needsEscapeReference)
		for _, limit := range []int{len(b), len(b) / 2} {
			buf := dst[:0:limit]
			got := QuoteCopy(buf, string(b))
			expect := min(want, limit)
			if got != expect {
				t.Fatalf("QuoteCopy(%q, cap %d) = %d, want %d", b, limit, got, expect)
			}
			if string(buf[:got]) != string(b[:got]) {
				t.Fatalf("QuoteCopy(%q) copied %q", b, buf[:got])
			}
		}
	})
}
