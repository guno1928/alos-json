//go:build amd64 && !purego

package scan

import "unsafe"

//go:noescape
func quoteEscapeAVX2(dst *byte, room int, src *byte, n int) (written, consumed int)

func QuoteEscapeAccelerated() bool {
	return useAVX2
}

func QuoteEscape(dst []byte, src string) (written, consumed int) {
	if !useAVX2 {
		return quoteEscapeGeneric(dst, src)
	}
	room := cap(dst) - len(dst)
	return quoteEscapeAVX2(unsafe.SliceData(dst[len(dst):cap(dst)]), room, unsafe.StringData(src), len(src))
}
