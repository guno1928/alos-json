//go:build amd64 && !purego

package scan

import (
	"math"

	"golang.org/x/sys/cpu"
)

const indexedValidMinLen = 4096

var useValidKernel = useAVX2 && hasBMI && cpu.X86.HasPCLMULQDQ && cpu.X86.HasPOPCNT

//go:noescape
func validBlocksStrict(p *byte, n int, st *blockState, out *uint32, outCap int) int

func Valid(b []byte) bool {
	if useValidKernel && len(b) >= indexedValidMinLen && len(b) <= math.MaxInt32 {
		return validAdaptive(b)
	}
	return validFast(b)
}
