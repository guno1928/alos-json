//go:build linux

package scan

import (
	"bytes"
	"syscall"
	"testing"
	"unsafe"
)

func newGuardedPage(t *testing.T) []byte {
	t.Helper()
	page := syscall.Getpagesize()
	mem, err := syscall.Mmap(-1, 0, 3*page, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := syscall.Munmap(mem); err != nil {
			t.Error(err)
		}
	})
	if err := syscall.Mprotect(mem[:page], syscall.PROT_NONE); err != nil {
		t.Fatal(err)
	}
	if err := syscall.Mprotect(mem[2*page:], syscall.PROT_NONE); err != nil {
		t.Fatal(err)
	}
	return mem[page : 2*page : 2*page]
}

func TestQuoteEscapeNeverReadsOutsideSourcePage(t *testing.T) {
	body := newGuardedPage(t)
	for n := 0; n <= quoteMaxTestLen; n++ {
		for _, c := range quoteSampleBytes() {
			data := bytes.Repeat([]byte{c}, n)
			if n > 2 {
				data[n/2] = '"'
			}
			checkGuardedQuoteEscape(t, body[len(body)-copy(body[len(body)-n:], data):])
			checkGuardedQuoteEscape(t, body[:copy(body, data)])
		}
	}
}

func checkGuardedQuoteEscape(t *testing.T, src []byte) {
	t.Helper()
	want := referenceQuoteEscape(string(src))
	out := make([]byte, 0, len(want)+MaxQuoteEscapeLen)
	written, consumed := QuoteEscape(out, unsafe.String(unsafe.SliceData(src), len(src)))
	if consumed != len(src) || !bytes.Equal(out[:written], want) {
		t.Fatalf("QuoteEscape(%q) = %q (consumed %d), want %q", src, out[:written], consumed, want)
	}
}
