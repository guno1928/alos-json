package jsontext

import "github.com/guno1928/alos-json/internal/scan"

// QuoteCopy performs a fused scan+copy: copies safe JSON string bytes from src to dst,
// stopping at the first byte that needs escaping (< 0x20, '"', '\').
// Returns the number of bytes copied. dst must have at least len(src) capacity available.
func QuoteCopy(dst []byte, src string) int {
	return scan.QuoteCopy(dst, src)
}
