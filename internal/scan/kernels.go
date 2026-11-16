package scan

import "unsafe"

func addPtr(p *byte, offset int) unsafe.Pointer {
	return unsafe.Add(unsafe.Pointer(p), offset)
}

func quoteCopyGeneric(src *byte, srcLen int, dst *byte, dstCap int) int {
	n := min(srcLen, dstCap)
	for i := 0; i < n; i++ {
		c := *(*byte)(addPtr(src, i))
		if c < 0x20 || c == '"' || c == '\\' {
			return i
		}
		*(*byte)(addPtr(dst, i)) = c
	}
	return n
}

func FindSafeString(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	return findSafe(&b[0], len(b))
}

func FindSafeStringHTML(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	return findSafeStringHTMLSIMD(&b[0], len(b))
}

func IndexBackslash(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	return countBackslashSIMD(&b[0], len(b))
}

func QuoteCopy(dst []byte, src string) int {
	n := len(src)
	avail := cap(dst) - len(dst)
	if n == 0 || avail <= 0 {
		return 0
	}
	limit := min(n, avail)
	return quoteCopy(unsafe.StringData(src), limit, unsafe.SliceData(dst[len(dst):]))
}
