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

func hasCustomMarshaler(t reflect.Type) bool {
	pt := reflect.PointerTo(t)
	return t.Implements(marshalerToType) || pt.Implements(marshalerToType) ||
		t.Implements(marshalerType) || pt.Implements(marshalerType) ||
		t.Implements(textMarshalerType) || pt.Implements(textMarshalerType)
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

const shortStringLen = 32

func appendJStr(dst []byte, s string) []byte {
	if len(s) <= shortStringLen {
		if out, ok := appendShortString(dst, s); ok {
			return out
		}
	}
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

func appendShortString(dst []byte, s string) ([]byte, bool) {
	start := len(dst)
	dst = slices.Grow(dst, len(s)+2)[:start+len(s)+2]
	out := dst[start+1 : start+1+len(s)]
	src := unsafe.Slice(unsafe.StringData(s), len(s))
	i := 0
	for ; i+8 <= len(src); i += 8 {
		w := binary.LittleEndian.Uint64(src[i:])
		if scan.SpecialMask(w) != 0 {
			return dst[:start], false
		}
		binary.LittleEndian.PutUint64(out[i:], w)
	}
	for ; i < len(src); i++ {
		c := src[i]
		if c < 0x20 || c == '"' || c == '\\' {
			return dst[:start], false
		}
		out[i] = c
	}
	dst[start] = '"'
	dst[len(dst)-1] = '"'
	return dst, true
}

func unsafeBytesFromStr(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}

const hexDigitsMarshal = "0123456789abcdef"

func hx(c byte) byte {
	return hexDigitsMarshal[c]
}

func mapKeyString(v reflect.Value) string {
	switch v.Kind() {
	case reflect.String:
		return v.String()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10)
	default:
		return ""
	}
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
