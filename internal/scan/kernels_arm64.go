//go:build arm64 && alosneon && !purego

package scan

//go:noescape
func findSafeStringSIMD(p *byte, n int) int

//go:noescape
func findSafeStringHTMLSIMD(p *byte, n int) int

//go:noescape
func countBackslashSIMD(p *byte, n int) int

func quoteCopySIMD(src *byte, srcLen int, dst *byte, dstCap int) int {
	return quoteCopyGeneric(src, srcLen, dst, dstCap)
}

func findSafe(p *byte, n int) int {
	return findSafeStringSIMD(p, n)
}

func quoteCopy(src *byte, n int, dst *byte) int {
	return quoteCopySIMD(src, n, dst, n)
}

func SpecialMask64(b []byte) uint64 {
	return specialMask64Generic(b)
}
