//go:build !amd64 || purego

package scan

func escapedStringEnd(b []byte, pos *int) (end int, done, ok bool) {
	return escapedStringEndScalar(b, pos)
}
