package jsontext

import (
	"bytes"
	"io"
	"testing"
)

const borrowedBufCap = 4096

func sentinelBuffer() []byte {
	return bytes.Repeat([]byte{'Z'}, borrowedBufCap)[:0]
}

func reuseEncoderPool(t *testing.T) {
	t.Helper()
	for range 200 {
		enc := NewEncoder(io.Discard)
		if err := enc.WriteString(string(bytes.Repeat([]byte{'x'}, 300))); err != nil {
			t.Fatal(err)
		}
		PutEncoder(enc)
	}
}

func assertUntouched(t *testing.T, buf []byte) {
	t.Helper()
	full := buf[:cap(buf)]
	if i := bytes.IndexFunc(full, func(r rune) bool { return r != 'Z' }); i >= 0 {
		t.Fatalf("pooled encoder wrote into a buffer it did not own at offset %d", i)
	}
}

func TestPutEncoderReleasesWrappedBuffer(t *testing.T) {
	borrowed := sentinelBuffer()
	for range 200 {
		PutEncoder(WrapBuffer(borrowed))
	}
	reuseEncoderPool(t)
	assertUntouched(t, borrowed)
}

func TestPutEncoderReleasesCallerBuffer(t *testing.T) {
	borrowed := sentinelBuffer()
	for range 200 {
		buf := borrowed
		PutEncoder(NewEncoderToBytes(&buf))
	}
	reuseEncoderPool(t)
	assertUntouched(t, borrowed)
}
