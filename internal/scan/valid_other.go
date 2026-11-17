//go:build !amd64 || purego

package scan

func Valid(b []byte) bool {
	return validFast(b)
}

func ValidLax(b []byte) bool {
	return validFastLax(b)
}
