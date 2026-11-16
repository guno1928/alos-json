//go:build !amd64 || purego

package scan

func escapedStringEnd(b []byte, pos *int) (end int, done, ok bool) {
	return escapedStringEndScalar(b, pos)
}

func escapedStringValid(b []byte, i int) (int, bool) {
	end, _, ok := StringEnd(b, i)
	return end, ok
}
