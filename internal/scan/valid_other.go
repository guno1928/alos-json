//go:build !amd64 || purego

package scan

func Valid(b []byte) bool {
	return validFast(b)
}
