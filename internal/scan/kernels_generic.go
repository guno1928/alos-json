//go:build (!amd64 || purego) && !(arm64 && alosneon && !purego)

package scan

func findSafeStringSIMD(p *byte, n int) int {
	for i := 0; i < n; i++ {
		c := *(*byte)(addPtr(p, i))
		if c < 0x20 || c == '"' || c == '\\' {
			return i
		}
	}
	return n
}

func findSafeStringHTMLSIMD(p *byte, n int) int {
	for i := 0; i < n; i++ {
		c := *(*byte)(addPtr(p, i))
		if c < 0x20 || c == '"' || c == '\\' || c == '<' || c == '>' || c == '&' {
			return i
		}
	}
	return n
}

func countBackslashSIMD(p *byte, n int) int {
	for i := 0; i < n; i++ {
		if *(*byte)(addPtr(p, i)) == '\\' {
			return i
		}
	}
	return n
}

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
