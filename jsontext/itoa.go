package jsontext

var digitPairs [200]byte

func init() {
	for i := 0; i < 100; i++ {
		digitPairs[i*2] = byte('0' + i/10)
		digitPairs[i*2+1] = byte('0' + i%10)
	}
}

func appendUint64(dst []byte, v uint64) []byte {
	if v < 10 {
		return append(dst, byte('0'+v))
	}
	if v < 100 {
		return append(dst, digitPairs[v*2], digitPairs[v*2+1])
	}

	var buf [20]byte
	pos := len(buf)

	for v >= 100 {
		idx := (v % 100) * 2
		v /= 100
		pos -= 2
		buf[pos] = digitPairs[idx]
		buf[pos+1] = digitPairs[idx+1]
	}
	if v >= 10 {
		pos -= 2
		buf[pos] = digitPairs[v*2]
		buf[pos+1] = digitPairs[v*2+1]
	} else {
		pos--
		buf[pos] = byte('0' + v)
	}
	return append(dst, buf[pos:]...)
}

func appendInt64(dst []byte, v int64) []byte {
	if v >= 0 {
		return appendUint64(dst, uint64(v))
	}
	dst = append(dst, '-')
	return appendUint64(dst, uint64(-v))
}

// FastAppendInt appends a decimal int64 to dst without allocations.
func FastAppendInt(dst []byte, v int64) []byte {
	return appendInt64(dst, v)
}

// FastAppendUint appends a decimal uint64 to dst without allocations.
func FastAppendUint(dst []byte, v uint64) []byte {
	return appendUint64(dst, v)
}
