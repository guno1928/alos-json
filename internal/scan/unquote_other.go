//go:build !amd64 || purego

package scan

func unquoteFast(dst, _ []byte, i int) ([]byte, int, bool) {
	return dst, i, false
}
