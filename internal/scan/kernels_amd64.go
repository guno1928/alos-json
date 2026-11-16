//go:build amd64 && !purego

package scan

import "golang.org/x/sys/cpu"

const avx2MinLen = 64

var useAVX2 = cpu.X86.HasAVX2

//go:noescape
func findSafeStringSIMD(p *byte, n int) int

//go:noescape
func findSafeStringHTMLSIMD(p *byte, n int) int

//go:noescape
func countBackslashSIMD(p *byte, n int) int

//go:noescape
func quoteCopySIMD(src *byte, srcLen int, dst *byte, dstCap int) int

//go:noescape
func findSafeStringAVX2(p *byte, n int) int

//go:noescape
func quoteCopyAVX2(src *byte, n int, dst *byte) int

//go:noescape
func specialMask64AVX2(p *byte) uint64

func SpecialMask64(b []byte) uint64 {
	if useAVX2 {
		_ = b[BlockLen-1]
		return specialMask64AVX2(&b[0])
	}
	return specialMask64Generic(b)
}

func findSafe(p *byte, n int) int {
	if n <= avx2MinLen || !useAVX2 {
		return findSafeStringSIMD(p, n)
	}
	return findSafeLong(p, n)
}

func findSafeLong(p *byte, n int) int {
	head := avx2MinLen
	if i := findSafeStringSIMD(p, head); i < head {
		return i
	}
	rest := n - head
	blocks := rest &^ 31
	j := head + findSafeStringAVX2((*byte)(addPtr(p, head)), blocks)
	if j < head+blocks || j == n {
		return j
	}
	return j + findSafeStringSIMD((*byte)(addPtr(p, j)), n-j)
}

func quoteCopy(src *byte, n int, dst *byte) int {
	if n <= avx2MinLen || !useAVX2 {
		return quoteCopySIMD(src, n, dst, n)
	}
	return quoteCopyLong(src, n, dst)
}

func quoteCopyLong(src *byte, n int, dst *byte) int {
	head := avx2MinLen
	if i := quoteCopySIMD(src, head, dst, head); i < head {
		return i
	}
	rest := n - head
	j := head + quoteCopyAVX2((*byte)(addPtr(src, head)), rest&^31, (*byte)(addPtr(dst, head)))
	if j == n {
		return j
	}
	return j + quoteCopySIMD((*byte)(addPtr(src, j)), n-j, (*byte)(addPtr(dst, j)), n-j)
}
