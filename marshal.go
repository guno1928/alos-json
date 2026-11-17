package json

import (
	"encoding"
	"encoding/binary"
	"math"
	"math/bits"
	"reflect"
	"slices"
	"strconv"
	"unsafe"

	"github.com/guno1928/alos-json/internal/fnum"
	"github.com/guno1928/alos-json/internal/scan"
	"github.com/guno1928/alos-json/jsontext"
)

var (
	marshalerToType   = reflect.TypeFor[MarshalerTo]()
	marshalerType     = reflect.TypeFor[Marshaler]()
	textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()
)

var (
	float32Type = reflect.TypeFor[float32]()
	float64Type = reflect.TypeFor[float64]()
)

func appendFloat(dst *[]byte, f float64, bits int) error {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		t := float64Type
		if bits == 32 {
			t = float32Type
		}
		return newSemanticError(t, "unsupported float value "+strconv.FormatFloat(f, 'g', -1, 64))
	}
	*dst = jsontext.FastAppendFloat(*dst, f, bits)
	return nil
}

func floatWindow(b []byte) *[fnum.FloatWindow]byte {
	return (*[fnum.FloatWindow]byte)(b[len(b) : len(b)+fnum.FloatWindow])
}

func appendFloat64(b []byte, f float64) ([]byte, error) {
	b = slices.Grow(b, fnum.FloatWindow)
	if n := fnum.PutFloat64(floatWindow(b), f); n != 0 {
		return b[:len(b)+n], nil
	}
	err := appendFloat(&b, f, 64)
	return b, err
}

func hasCustomMarshaler(t reflect.Type) bool {
	pt := reflect.PointerTo(t)
	return t.Implements(marshalerToType) || pt.Implements(marshalerToType) ||
		t.Implements(marshalerType) || pt.Implements(marshalerType) ||
		t.Implements(textMarshalerType) || pt.Implements(textMarshalerType) ||
		isStdNumberType(t)
}

type eface struct {
	rtype unsafe.Pointer
	data  unsafe.Pointer
}

func efaceOf(v *any) eface {
	return *(*eface)(unsafe.Pointer(v))
}

// Marshal encodes v as JSON and returns the result.
func Marshal(v any, opts ...Option) ([]byte, error) {
	return marshalFast(v, opts)
}

// MarshalAppend appends the JSON encoding of v to dst.
func MarshalAppend(dst []byte, v any, opts ...Option) ([]byte, error) {
	return marshalAppendFast(dst, v, opts)
}

const (
	shortStringLen    = 32
	quotedStringSlack = 2 + scan.MaxQuoteEscapeLen
	escapeRegrowRatio = 2
)

func appendJStr(dst []byte, s string) []byte {
	return appendQuotedString(dst, s, false)
}

func appendQuotedSlow(dst []byte, s string, escapeHTML bool) []byte {
	if escapeHTML {
		return jsontext.AppendQuoteStringHTML(dst, s)
	}
	if scan.QuoteEscapeAccelerated() {
		return appendQuotedKernel(dst, s)
	}
	return appendQuotedPortable(dst, s)
}

func appendQuotedKernel(dst []byte, s string) []byte {
	dst = slices.Grow(dst, len(s)+quotedStringSlack)
	dst = append(dst, '"')
	for len(s) > 0 {
		written, consumed := scan.QuoteEscape(dst, s)
		dst = dst[:len(dst)+written]
		s = s[consumed:]
		if len(s) == 0 {
			break
		}
		dst = slices.Grow(dst, escapeRegrowRatio*len(s)+quotedStringSlack)
	}
	return append(dst, '"')
}

func appendQuotedPortable(dst []byte, s string) []byte {
	dst = append(dst, '"')
	for len(s) > 0 {
		// Ensure dst has enough capacity for the remaining string
		dst = slices.Grow(dst, len(s))
		// Fused scan+copy: copies safe bytes and returns count
		n := jsontext.QuoteCopy(dst, s)
		dst = dst[:len(dst)+n]
		s = s[n:]
		if len(s) == 0 {
			break
		}
		// Handle the escape character
		dst = appendEscapedByte(dst, s[0])
		s = s[1:]
		dst, s = appendEscapedBlocks(dst, s)
	}
	dst = append(dst, '"')
	return dst
}

func appendEscapedBlocks(dst []byte, s string) ([]byte, string) {
	for len(s) >= scan.BlockLen {
		m := scan.SpecialMask64(unsafe.Slice(unsafe.StringData(s), scan.BlockLen))
		if m == 0 {
			return dst, s
		}
		last := 0
		for m != 0 {
			off := bits.TrailingZeros64(m)
			m &= m - 1
			dst = append(dst, s[last:off]...)
			dst = appendEscapedByte(dst, s[off])
			last = off + 1
		}
		dst = append(dst, s[last:scan.BlockLen]...)
		s = s[scan.BlockLen:]
	}
	return dst, s
}

func appendEscapedByte(dst []byte, c byte) []byte {
	switch c {
	case '"':
		return append(dst, '\\', '"')
	case '\\':
		return append(dst, '\\', '\\')
	case '\n':
		return append(dst, '\\', 'n')
	case '\r':
		return append(dst, '\\', 'r')
	case '\t':
		return append(dst, '\\', 't')
	case '\b':
		return append(dst, '\\', 'b')
	case '\f':
		return append(dst, '\\', 'f')
	}
	return append(dst, '\\', 'u', '0', '0', hx(c>>4), hx(c&0xF))
}

const (
	wordLen        = 8
	halfWordLen    = 4
	byteBits       = 8
	halfWordBits   = halfWordLen * byteBits
	safeUpperLanes = 0x4141414141000000
)

func appendQuotedString(buf []byte, s string, escapeHTML bool) []byte {
	n := len(s)
	if n > shortStringLen || escapeHTML {
		return appendQuotedSlow(buf, s, escapeHTML)
	}
	start := len(buf)
	if cap(buf)-start < n+2 {
		buf = slices.Grow(buf, n+2)
	}
	dst := buf[:start+n+2]
	out := dst[start+1 : start+1+n]
	src := unsafe.Slice(unsafe.StringData(s), n)
	var special uint64
	switch {
	case n >= wordLen:
		for i := 0; i < n-wordLen; i += wordLen {
			w := binary.LittleEndian.Uint64(src[i:])
			special |= scan.SpecialMask(w)
			binary.LittleEndian.PutUint64(out[i:], w)
		}
		w := binary.LittleEndian.Uint64(src[n-wordLen:])
		special |= scan.SpecialMask(w)
		binary.LittleEndian.PutUint64(out[n-wordLen:], w)
	case n >= halfWordLen:
		lo := binary.LittleEndian.Uint32(src)
		hi := binary.LittleEndian.Uint32(src[n-halfWordLen:])
		special = scan.SpecialMask(uint64(lo) | uint64(hi)<<halfWordBits)
		binary.LittleEndian.PutUint32(out, lo)
		binary.LittleEndian.PutUint32(out[n-halfWordLen:], hi)
	case n > 0:
		first, mid, last := src[0], src[n/2], src[n-1]
		special = scan.SpecialMask(uint64(first) | uint64(mid)<<byteBits | uint64(last)<<(2*byteBits) | safeUpperLanes)
		out[0], out[n/2], out[n-1] = first, mid, last
	}
	if special != 0 {
		return appendQuotedSlow(buf, s, false)
	}
	dst[start] = '"'
	dst[len(dst)-1] = '"'
	return dst
}

func unsafeBytesFromStr(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

const hexDigitsMarshal = "0123456789abcdef"

func hx(c byte) byte {
	return hexDigitsMarshal[c]
}

func appendMapKeyStr(dst []byte, v reflect.Value) []byte {
	switch v.Kind() {
	case reflect.String:
		return appendJStr(dst, v.String())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		dst = append(dst, '"')
		dst = jsontext.FastAppendInt(dst, v.Int())
		dst = append(dst, '"')
		return dst
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		dst = append(dst, '"')
		dst = jsontext.FastAppendUint(dst, v.Uint())
		dst = append(dst, '"')
		return dst
	default:
		dst = append(dst, '"', '"')
		return dst
	}
}

func fieldByIndex(v reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				return reflect.Value{}
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v
}

func isZero(v reflect.Value) bool {
	if v.CanAddr() {
		if z, ok := v.Addr().Interface().(IsZeroer); ok {
			return z.IsZero()
		}
	}
	return v.IsZero()
}

func isEmpty(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}

func b2s(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func isDirectIface(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return true
	case reflect.Array:
		return t.Len() == 1 && isDirectIface(t.Elem())
	case reflect.Struct:
		return t.NumField() == 1 && isDirectIface(t.Field(0).Type)
	}
	return false
}

func growSlice(b []byte, n int) []byte {
	need := len(b) + n
	if cap(b) >= need {
		return b[:need]
	}
	nb := make([]byte, need, need*2)
	copy(nb, b)
	return nb
}
