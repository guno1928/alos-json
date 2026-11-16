//go:build amd64 && !purego

package scan

import "slices"

const unquoteKernelSlack = 64

//go:noescape
func unquoteAVX2(dst *byte, dstFree int, src *byte, srcLen int) (written, consumed, closed int)

func unquoteFast(dst, b []byte, i int) ([]byte, int, bool) {
	if !useAVX2 || len(b)-i < unquoteKernelSlack {
		return dst, i, false
	}
	dst = slices.Grow(dst, unquoteKernelSlack)
	n := len(dst)
	written, consumed, closed := unquoteAVX2(&dst[:cap(dst)][n], cap(dst)-n, &b[i], len(b)-i)
	return dst[:n+written], i + consumed, closed != 0
}
