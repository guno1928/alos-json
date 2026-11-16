//go:build !amd64 || purego

package scan

func QuoteEscapeAccelerated() bool {
	return false
}

func QuoteEscape(dst []byte, src string) (written, consumed int) {
	return quoteEscapeGeneric(dst, src)
}
