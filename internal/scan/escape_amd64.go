//go:build amd64 && !purego

package scan

import "golang.org/x/sys/cpu"

var hasBMI = cpu.X86.HasBMI1 && cpu.X86.HasBMI2

func useEscapeKernel() bool { return useAVX2 && hasBMI }

//go:noescape
func escapedStringEndAVX2(p *byte, n int) int

func escapedStringEnd(b []byte, pos *int) (end int, done, ok bool) {
	if !useEscapeKernel() {
		return escapedStringEndScalar(b, pos)
	}
	i := *pos
	if len(b)-i >= BlockLen {
		if r := escapedStringEndAVX2(&b[i], len(b)-i); r >= 0 {
			return i + r, true, true
		}
	}
	end, ok = scalarStringEnd(b, i)
	return end, true, ok
}

func escapedStringValid(b []byte, i int) (int, bool) {
	if useEscapeKernel() && len(b)-i >= BlockLen {
		if r := escapedStringEndAVX2(&b[i], len(b)-i); r >= 0 {
			return i + r, true
		}
	}
	return scalarStringEnd(b, i)
}

func scalarStringEnd(b []byte, i int) (int, bool) {
	for {
		i = NextSpecial(b, i)
		if i >= len(b) {
			return i, false
		}
		switch b[i] {
		case '"':
			return i, true
		case '\\':
			n := escapeLen(b, i)
			if n == 0 {
				return i, false
			}
			i += n
		default:
			return i, false
		}
	}
}
