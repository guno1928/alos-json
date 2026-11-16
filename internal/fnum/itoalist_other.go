//go:build !amd64 || purego

package fnum

func IntListKernelUseful(int, uintptr) bool {
	return false
}

func AppendIntListKernel[T int | int8 | int16 | int32 | int64](dst []byte, vs []T) []byte {
	panic("fnum: integer list kernel unavailable")
}

func AppendUintListKernel[T uint | uint16 | uint32 | uint64](dst []byte, vs []T) []byte {
	panic("fnum: integer list kernel unavailable")
}
