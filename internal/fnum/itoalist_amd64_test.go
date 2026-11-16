//go:build amd64 && !purego

package fnum

import (
	"bytes"
	"fmt"
	"testing"

	"golang.org/x/sys/cpu"
)

const kernelProbeOffsets = 16

type listKernel struct {
	name    string
	enabled bool
	ints    func(dst *byte, src *int64, n int) int
	uints   func(dst *byte, src *uint64, n int) int
}

var listKernels = []listKernel{
	{"SSSE3", cpu.X86.HasSSSE3, formatInt64sSSSE3, formatUint64sSSSE3},
	{"AVX2", cpu.X86.HasAVX2, formatInt64sAVX2, formatUint64sAVX2},
}

func checkListKernels(t *testing.T, ints []int64, uints []uint64) {
	t.Helper()
	for _, k := range listKernels {
		if !k.enabled {
			continue
		}
		if len(ints) > 0 {
			runKernelProbe(t, k.name, len(ints), referenceInt64List(nil, ints), func(dst *byte) int {
				return k.ints(dst, &ints[0], len(ints))
			})
		}
		if len(uints) > 0 {
			runKernelProbe(t, k.name, len(uints), referenceUint64List(nil, uints), func(dst *byte) int {
				return k.uints(dst, &uints[0], len(uints))
			})
		}
	}
}

func runKernelProbe(t *testing.T, name string, n int, want []byte, kernel func(dst *byte) int) {
	t.Helper()
	reserve := n*maxListEntryBytes + listStoreSlack
	for off := range kernelProbeOffsets {
		backing := bytes.Repeat([]byte{sentinel}, off+reserve+kernelProbeOffsets)
		w := kernel(&backing[off])
		if w != len(want) || !bytes.Equal(backing[off:off+w], want) {
			t.Fatalf("%s kernel(n=%d, off=%d) wrote %q, want %q", name, n, off, backing[off:off+max(w, 0)], want)
		}
		for i, c := range backing[:off] {
			if c != sentinel {
				t.Fatalf("%s kernel(n=%d, off=%d) wrote before dst at %d", name, n, off, i)
			}
		}
		for i, c := range backing[off+w+listStoreSlack:] {
			if c != sentinel {
				t.Fatalf("%s kernel(n=%d, off=%d) wrote %d bytes past slack", name, n, off, i+1)
			}
		}
	}
}

func withAVX2ListKernel(enabled bool, fn func()) {
	saved := useAVX2ListKernel
	useAVX2ListKernel = enabled
	defer func() { useAVX2ListKernel = saved }()
	fn()
}

func TestIntegerListKernelUseful(t *testing.T) {
	t.Logf("SSSE3=%v AVX2=%v", cpu.X86.HasSSSE3, cpu.X86.HasAVX2)
	if IntListKernelUseful(listKernelMinLen-1, wordSize) || IntListKernelUseful(listKernelMinLen, 1) {
		t.Fatal("kernel must not be used for short lists or byte-sized elements")
	}
	if got := IntListKernelUseful(listKernelMinLen, wordSize); got != cpu.X86.HasSSSE3 {
		t.Fatalf("IntListKernelUseful(%d, %d) = %v, want %v", listKernelMinLen, wordSize, got, cpu.X86.HasSSSE3)
	}
}

func TestIntegerListDispatchPaths(t *testing.T) {
	requireListKernel(t)
	ints, uints := edgeInt64s(), edgeUint64s()
	narrow := []int32{-2147483648, 2147483647, 0, -1, 7, 99999999, -100000000}
	for _, avx2 := range []bool{false, cpu.X86.HasAVX2} {
		withAVX2ListKernel(avx2, func() {
			if got, want := AppendIntListKernel([]byte("["), ints), referenceInt64List([]byte("["), ints); !bytes.Equal(got, want) {
				t.Fatalf("avx2=%v int64 list mismatch", avx2)
			}
			if got, want := AppendUintListKernel(nil, uints), referenceUint64List(nil, uints); !bytes.Equal(got, want) {
				t.Fatalf("avx2=%v uint64 list mismatch", avx2)
			}
			if got, want := string(AppendIntListKernel(nil, narrow)), "-2147483648,2147483647,0,-1,7,99999999,-100000000,"; got != want {
				t.Fatalf("avx2=%v int32 list = %q, want %q", avx2, got, want)
			}
		})
	}
}

func BenchmarkInt64ListPaths(b *testing.B) {
	requireListKernel(b)
	for _, n := range []int{1, 2, 3, 4, 8, 16, 64, 10000} {
		vs := make([]int64, n)
		for i := range vs {
			vs[i] = int64(i*7919) - int64(i%3)*1_000_003
		}
		for _, avx2 := range []bool{false, true} {
			b.Run(fmt.Sprintf("n=%d/avx2=%v", n, avx2), func(b *testing.B) {
				withAVX2ListKernel(avx2 && cpu.X86.HasAVX2, func() {
					dst := make([]byte, 0, 64<<10)
					for b.Loop() {
						dst = AppendIntListKernel(dst[:0], vs)
					}
				})
			})
		}
	}
}
