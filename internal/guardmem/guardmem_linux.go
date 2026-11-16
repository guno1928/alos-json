//go:build linux

package guardmem

import "syscall"

type Region struct {
	mem []byte
}

func Place(data []byte) (*Region, []byte, error) {
	page := syscall.Getpagesize()
	body := (len(data) + page - 1) / page * page
	if body == 0 {
		body = page
	}
	mem, err := syscall.Mmap(-1, 0, body+page, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_ANON|syscall.MAP_PRIVATE)
	if err != nil {
		return nil, nil, err
	}
	if err := syscall.Mprotect(mem[body:], syscall.PROT_NONE); err != nil {
		syscall.Munmap(mem)
		return nil, nil, err
	}
	start := body - len(data)
	copy(mem[start:body], data)
	return &Region{mem: mem}, mem[start:body:body], nil
}

func (r *Region) Release() error {
	return syscall.Munmap(r.mem)
}
