//go:build amd64 && !purego

package scan

import "testing"

func TestKernelDispatchWithoutAVX2(t *testing.T) {
	saved := useAVX2
	useAVX2 = false
	defer func() { useAVX2 = saved }()
	checkDispatch(t)
}

func TestAVX2Available(t *testing.T) {
	t.Logf("AVX2 dispatch enabled: %v", useAVX2)
}
