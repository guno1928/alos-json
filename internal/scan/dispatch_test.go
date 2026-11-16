package scan

import "testing"

const dispatchMaxLen = 200

var dispatchCriticalBytes = []byte{0x00, 0x1f, 0x20, '"', '\\', '<', 0x7f, 0x80, 0xff}

func forEachDispatchInput(t *testing.T, check func(t *testing.T, b []byte)) {
	backing := make([]byte, kernelMaxOffset+dispatchMaxLen+64)
	for offset := range kernelMaxOffset {
		for n := 0; n <= dispatchMaxLen; n++ {
			b := backing[offset : offset+n]
			for i := range b {
				b[i] = kernelFillerByte
			}
			check(t, b)
			for pos := range n {
				for _, c := range dispatchCriticalBytes {
					b[pos] = c
					check(t, b)
				}
				b[pos] = kernelFillerByte
			}
		}
	}
}

func checkDispatch(t *testing.T) {
	dst := make([]byte, 0, dispatchMaxLen+kernelMaxOffset)
	forEachDispatchInput(t, func(t *testing.T, b []byte) {
		want := firstIndexReference(b, needsEscapeReference)
		if got := FindSafeString(b); got != want {
			t.Fatalf("FindSafeString(len %d) = %d, want %d", len(b), got, want)
		}
		if got := NextSpecial(b, 0); got != want {
			t.Fatalf("NextSpecial(len %d) = %d, want %d", len(b), got, want)
		}
		got := QuoteCopy(dst[:0:len(b)], string(b))
		if got != want {
			t.Fatalf("QuoteCopy(len %d) = %d, want %d", len(b), got, want)
		}
		if string(dst[:got]) != string(b[:got]) {
			t.Fatalf("QuoteCopy(len %d) copied wrong bytes", len(b))
		}
	})
}

func TestKernelDispatchMatchesReference(t *testing.T) {
	checkDispatch(t)
}
