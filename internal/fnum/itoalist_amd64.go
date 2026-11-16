//go:build amd64 && !purego

package fnum

import (
	"slices"
	"unsafe"

	"golang.org/x/sys/cpu"
)

const (
	maxListEntryBytes = 22
	listStoreSlack    = 16
	listChunkLen      = 512
	listKernelMinLen  = 4
	wideningChunkLen  = 256
	wordSize          = 8
)

var (
	useListKernel     = cpu.X86.HasSSSE3
	useAVX2ListKernel = cpu.X86.HasAVX2
)

//go:noescape
func formatInt64sSSSE3(dst *byte, src *int64, n int) int

//go:noescape
func formatUint64sSSSE3(dst *byte, src *uint64, n int) int

//go:noescape
func formatInt64sAVX2(dst *byte, src *int64, n int) int

//go:noescape
func formatUint64sAVX2(dst *byte, src *uint64, n int) int

func IntListKernelUseful(n int, elemSize uintptr) bool {
	return useListKernel && n >= listKernelMinLen && elemSize > 1
}

func AppendIntListKernel[T int | int8 | int16 | int32 | int64](dst []byte, vs []T) []byte {
	var zero T
	if unsafe.Sizeof(zero) == wordSize {
		return appendInt64Chunks(dst, unsafe.Slice((*int64)(unsafe.Pointer(unsafe.SliceData(vs))), len(vs)))
	}
	var wide [wideningChunkLen]int64
	for len(vs) > 0 {
		n := min(len(vs), len(wide))
		for i, v := range vs[:n] {
			wide[i] = int64(v)
		}
		dst = appendInt64Chunks(dst, wide[:n])
		vs = vs[n:]
	}
	return dst
}

func AppendUintListKernel[T uint | uint16 | uint32 | uint64](dst []byte, vs []T) []byte {
	var zero T
	if unsafe.Sizeof(zero) == wordSize {
		return appendUint64Chunks(dst, unsafe.Slice((*uint64)(unsafe.Pointer(unsafe.SliceData(vs))), len(vs)))
	}
	var wide [wideningChunkLen]uint64
	for len(vs) > 0 {
		n := min(len(vs), len(wide))
		for i, v := range vs[:n] {
			wide[i] = uint64(v)
		}
		dst = appendUint64Chunks(dst, wide[:n])
		vs = vs[n:]
	}
	return dst
}

func appendInt64Chunks(dst []byte, vs []int64) []byte {
	for len(vs) > 0 {
		n := min(len(vs), listChunkLen)
		dst = reserveListSpace(dst, n)
		dst = dst[:len(dst)+formatInt64s(spareStart(dst), vs[:n])]
		vs = vs[n:]
	}
	return dst
}

func appendUint64Chunks(dst []byte, vs []uint64) []byte {
	for len(vs) > 0 {
		n := min(len(vs), listChunkLen)
		dst = reserveListSpace(dst, n)
		dst = dst[:len(dst)+formatUint64s(spareStart(dst), vs[:n])]
		vs = vs[n:]
	}
	return dst
}

func formatInt64s(dst *byte, vs []int64) int {
	if useAVX2ListKernel {
		return formatInt64sAVX2(dst, &vs[0], len(vs))
	}
	return formatInt64sSSSE3(dst, &vs[0], len(vs))
}

func formatUint64s(dst *byte, vs []uint64) int {
	if useAVX2ListKernel {
		return formatUint64sAVX2(dst, &vs[0], len(vs))
	}
	return formatUint64sSSSE3(dst, &vs[0], len(vs))
}

func reserveListSpace(dst []byte, n int) []byte {
	return slices.Grow(dst, n*maxListEntryBytes+listStoreSlack)
}

func spareStart(dst []byte) *byte {
	return (*byte)(unsafe.Add(unsafe.Pointer(unsafe.SliceData(dst)), len(dst)))
}
