package json

import (
	"encoding"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"reflect"
	"strconv"
	"sync"
	"unsafe"

	"github.com/guno1928/alos-json/jsontext"
)

type directDec func(dec *jsontext.Decoder, v reflect.Value, o *Options) error

var directDecCache sync.Map

func cachedDirectDec(t reflect.Type) directDec {
	if v, ok := directDecCache.Load(t); ok {
		return v.(directDec)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var fn directDec
	fi, loaded := directDecCache.LoadOrStore(t, directDec(func(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
		wg.Wait()
		return fn(dec, v, o)
	}))
	if loaded {
		return fi.(directDec)
	}
	fn = buildDirectDec(t)
	directDecCache.Store(t, fn)
	wg.Done()
	return fn
}

var (
	unmarshalerFromType = reflect.TypeFor[UnmarshalerFrom]()
	unmarshalerType     = reflect.TypeFor[Unmarshaler]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

func buildDirectDec(t reflect.Type) directDec {
	pt := reflect.PointerTo(t)

	if pt.Implements(unmarshalerFromType) {
		return decUnmarshalerFrom
	}
	if pt.Implements(unmarshalerType) {
		return decUnmarshaler
	}
	if pt.Implements(textUnmarshalerType) {
		return decTextUnmarshaler
	}

	switch t.Kind() {
	case reflect.Bool:
		return decBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return decInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return decUint
	case reflect.Float32, reflect.Float64:
		return decFloat
	case reflect.String:
		return decString
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return decBytes
		}
		return decSlice
	case reflect.Array:
		return decArray
	case reflect.Map:
		return decMap
	case reflect.Struct:
		return decStruct
	case reflect.Interface:
		return decInterface
	case reflect.Pointer:
		return decPointer
	default:
		return decSkip
	}
}

func decUnmarshalerFrom(dec *jsontext.Decoder, v reflect.Value, _ *Options) error {
	if v.CanAddr() {
		return v.Addr().Interface().(UnmarshalerFrom).UnmarshalJSONFrom(dec)
	}
	return dec.SkipValue()
}

func decUnmarshaler(dec *jsontext.Decoder, v reflect.Value, _ *Options) error {
	if v.CanAddr() {
		val, err := dec.ReadValue()
		if err != nil {
			return err
		}
		return v.Addr().Interface().(Unmarshaler).UnmarshalJSON(val)
	}
	return dec.SkipValue()
}

func decTextUnmarshaler(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	if v.CanAddr() && dec.PeekKind() == jsontext.String {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		return v.Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(tok.Str()))
	}
	return unmarshalReflect(dec, v, o)
}

func decBool(dec *jsontext.Decoder, v reflect.Value, _ *Options) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	switch {
	case tok.Kind() == jsontext.True || tok.Kind() == jsontext.False:
		v.SetBool(tok.Bool())
	case tok.Kind() == jsontext.String && (tok.Str() == "true" || tok.Str() == "false"):
		v.SetBool(tok.Str() == "true")
	default:
		return newSemanticError(v.Type(), "expected bool")
	}
	return nil
}

func decInt(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	return unmarshalInt(dec, v, o)
}

func decUint(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	return unmarshalUint(dec, v, o)
}

func decFloat(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	return unmarshalFloat(dec, v, o)
}

func decString(dec *jsontext.Decoder, v reflect.Value, _ *Options) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	if tok.Kind() != jsontext.String {
		return newSemanticError(v.Type(), "expected string")
	}
	v.SetString(tok.Str())
	return nil
}

func decBytes(dec *jsontext.Decoder, v reflect.Value, _ *Options) error {
	return unmarshalBytes(dec, v)
}

func decSlice(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	return unmarshalSlice(dec, v, o)
}

func decArray(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	return unmarshalFixedArray(dec, v, o)
}

func decMap(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	return unmarshalMap(dec, v, o)
}

func decStruct(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	return unmarshalStruct(dec, v, o)
}

func decInterface(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	return unmarshalInterface(dec, v, o)
}

func decPointer(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	if v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	return unmarshalValue(dec, v.Elem(), o)
}

func decSkip(dec *jsontext.Decoder, _ reflect.Value, _ *Options) error {
	return dec.SkipValue()
}

// Unmarshal parses the JSON-encoded data and stores the result in the value pointed to by v.
func Unmarshal(data []byte, v any, opts ...Option) error {
	return unmarshalBytesFast(data, v, opts)
}

func unmarshalTopLevel(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	if err := unmarshalValue(dec, v, o); err != nil {
		return err
	}
	return expectEndOfInput(dec)
}

func expectEndOfInput(dec *jsontext.Decoder) error {
	offset := dec.InputOffset()
	_, err := dec.ReadToken()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return &jsontext.SyntacticError{ByteOffset: offset, Msg: "invalid data after top-level value"}
}

func unmarshalValue(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	k := dec.PeekKind()
	if k == jsontext.Invalid {
		return fmt.Errorf("json: unexpected end of input")
	}

	if k == jsontext.Null {
		dec.ReadToken()
		if v.CanSet() {
			v.Set(reflect.Zero(v.Type()))
		}
		return nil
	}

	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		v = v.Elem()
	}

	return cachedDirectDec(v.Type())(dec, v, o)
}

func unmarshalReflect(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	k := dec.PeekKind()

	switch v.Kind() {
	case reflect.Bool:
		return decBool(dec, v, o)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return unmarshalInt(dec, v, o)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return unmarshalUint(dec, v, o)

	case reflect.Float32, reflect.Float64:
		return unmarshalFloat(dec, v, o)

	case reflect.String:
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		if tok.Kind() != jsontext.String {
			return newSemanticError(v.Type(), "expected string")
		}
		v.SetString(tok.Str())
		return nil

	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return unmarshalBytes(dec, v)
		}
		return unmarshalSlice(dec, v, o)

	case reflect.Array:
		return unmarshalFixedArray(dec, v, o)

	case reflect.Map:
		return unmarshalMap(dec, v, o)

	case reflect.Struct:
		return unmarshalStruct(dec, v, o)

	case reflect.Interface:
		return unmarshalInterface(dec, v, o)

	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return unmarshalValue(dec, v.Elem(), o)

	default:
		_ = k
		return dec.SkipValue()
	}
}

func fastParseInt(s string) (int64, bool) {
	if len(s) == 0 || len(s) > 19 {
		return 0, false
	}
	neg := false
	i := 0
	if s[0] == '-' {
		neg = true
		i = 1
		if len(s) == 1 {
			return 0, false
		}
	}
	var n uint64
	for ; i < len(s); i++ {
		c := s[i] - '0'
		if c > 9 {
			return 0, false
		}
		n = n*10 + uint64(c)
	}
	if neg {
		if n > 1<<63 {
			return 0, false
		}
		return -int64(n), true
	}
	if n > uint64(1<<63-1) {
		return 0, false
	}
	return int64(n), true
}

const maxSafeUintDigits = 19

func fastParseUint(s string) (uint64, bool) {
	if len(s) == 0 || len(s) > maxSafeUintDigits {
		return 0, false
	}
	var n uint64
	for i := 0; i < len(s); i++ {
		c := s[i] - '0'
		if c > 9 {
			return 0, false
		}
		n = n*10 + uint64(c)
	}
	return n, true
}

const (
	twoPow63 = 9223372036854775808.0
	twoPow64 = 18446744073709551616.0
)

func isRangeError(err error) bool {
	ne, ok := err.(*strconv.NumError)
	return ok && ne.Err == strconv.ErrRange
}

func overflowError(t reflect.Type, raw string) error {
	return newSemanticError(t, "number "+raw+" overflows "+t.String())
}

func intFits(n int64, bits int) bool {
	return bits >= 64 || (n >= -(1<<(bits-1)) && n < 1<<(bits-1))
}

func uintFits(n uint64, bits int) bool {
	return bits >= 64 || n < 1<<bits
}

func parseIntValue(raw string, t reflect.Type, allowFraction bool) (int64, error) {
	n, ok := fastParseInt(raw)
	if !ok {
		var err error
		n, err = strconv.ParseInt(raw, 10, 64)
		switch {
		case err == nil:
		case isRangeError(err):
			return 0, overflowError(t, raw)
		case !allowFraction:
			return 0, newSemanticError(t, "cannot parse "+raw+" as "+t.String())
		default:
			f, ferr := strconv.ParseFloat(raw, 64)
			if ferr != nil {
				return 0, newSemanticError(t, "invalid number: "+raw)
			}
			f = math.Trunc(f)
			if !(f >= -twoPow63 && f < twoPow63) {
				return 0, overflowError(t, raw)
			}
			n = int64(f)
		}
	}
	if !intFits(n, t.Bits()) {
		return 0, overflowError(t, raw)
	}
	return n, nil
}

func parseUintValue(raw string, t reflect.Type, allowFraction bool) (uint64, error) {
	n, ok := fastParseUint(raw)
	if !ok {
		var err error
		n, err = strconv.ParseUint(raw, 10, 64)
		switch {
		case err == nil:
		case isRangeError(err):
			return 0, overflowError(t, raw)
		case !allowFraction:
			return 0, newSemanticError(t, "cannot parse "+raw+" as "+t.String())
		default:
			f, ferr := strconv.ParseFloat(raw, 64)
			if ferr != nil {
				return 0, newSemanticError(t, "invalid number: "+raw)
			}
			f = math.Trunc(f)
			if !(f >= 0 && f < twoPow64) {
				return 0, overflowError(t, raw)
			}
			n = uint64(f)
		}
	}
	if !uintFits(n, t.Bits()) {
		return 0, overflowError(t, raw)
	}
	return n, nil
}

func storeInt(p unsafe.Pointer, k reflect.Kind, n int64) {
	switch k {
	case reflect.Int:
		*(*int)(p) = int(n)
	case reflect.Int8:
		*(*int8)(p) = int8(n)
	case reflect.Int16:
		*(*int16)(p) = int16(n)
	case reflect.Int32:
		*(*int32)(p) = int32(n)
	default:
		*(*int64)(p) = n
	}
}

func storeUint(p unsafe.Pointer, k reflect.Kind, n uint64) {
	switch k {
	case reflect.Uint:
		*(*uint)(p) = uint(n)
	case reflect.Uint8:
		*(*uint8)(p) = uint8(n)
	case reflect.Uint16:
		*(*uint16)(p) = uint16(n)
	case reflect.Uint32:
		*(*uint32)(p) = uint32(n)
	case reflect.Uintptr:
		*(*uintptr)(p) = uintptr(n)
	default:
		*(*uint64)(p) = n
	}
}

func unmarshalInt(dec *jsontext.Decoder, v reflect.Value, _ *Options) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	switch tok.Kind() {
	case jsontext.Number, jsontext.String:
		n, err := parseIntValue(tokenText(tok), v.Type(), tok.Kind() == jsontext.Number)
		if err != nil {
			return err
		}
		v.SetInt(n)
		return nil
	default:
		return newSemanticError(v.Type(), "expected number, got "+tok.Kind().String())
	}
}

func unmarshalUint(dec *jsontext.Decoder, v reflect.Value, _ *Options) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	switch tok.Kind() {
	case jsontext.Number, jsontext.String:
		n, err := parseUintValue(tokenText(tok), v.Type(), tok.Kind() == jsontext.Number)
		if err != nil {
			return err
		}
		v.SetUint(n)
		return nil
	default:
		return newSemanticError(v.Type(), "expected number, got "+tok.Kind().String())
	}
}

func unmarshalFloat(dec *jsontext.Decoder, v reflect.Value, _ *Options) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	bits := 64
	if v.Type().Kind() == reflect.Float32 {
		bits = 32
	}
	switch tok.Kind() {
	case jsontext.Number:
		f, err := strconv.ParseFloat(tokenText(tok), bits)
		if err != nil {
			return newSemanticError(v.Type(), "invalid float: "+tokenText(tok))
		}
		v.SetFloat(f)
		return nil
	case jsontext.String:
		f, err := strconv.ParseFloat(tok.Str(), bits)
		if err != nil {
			return newSemanticError(v.Type(), "cannot parse string as float: "+tok.Str())
		}
		v.SetFloat(f)
		return nil
	default:
		return newSemanticError(v.Type(), "expected number, got "+tok.Kind().String())
	}
}

func tokenText(tok jsontext.Token) string {
	if tok.Kind() == jsontext.Number {
		return b2s(tok.Raw())
	}
	return tok.Str()
}

func parseAnyNumber(raw string) (any, error) {
	f, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, newSemanticError(float64Type, "number "+raw+" out of range for float64")
	}
	return f, nil
}

func unmarshalBytes(dec *jsontext.Decoder, v reflect.Value) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	if tok.Kind() == jsontext.Null {
		v.SetBytes(nil)
		return nil
	}
	if tok.Kind() != jsontext.String {
		return newSemanticError(v.Type(), "expected base64 string")
	}
	b, err := base64.StdEncoding.DecodeString(tok.Str())
	if err != nil {
		return newSemanticError(v.Type(), "invalid base64: "+err.Error())
	}
	v.SetBytes(b)
	return nil
}

func unmarshalSlice(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	if dec.PeekKind() != jsontext.BeginArray {
		return newSemanticError(v.Type(), "expected array")
	}
	dec.ReadToken()

	elemType := v.Type().Elem()
	elemDec := cachedDirectDec(elemType)
	reusedCap := v.Cap()
	i := 0
	for dec.More() {
		if i >= v.Cap() {
			newCap := v.Cap()*2 + 4
			nv := reflect.MakeSlice(v.Type(), v.Len(), newCap)
			reflect.Copy(nv, v)
			v.Set(nv)
		}
		if i >= v.Len() {
			v.SetLen(i + 1)
			if i < reusedCap {
				v.Index(i).SetZero()
			}
		}
		elem := v.Index(i)
		pk := dec.PeekKind()
		if pk == jsontext.Null {
			dec.ReadToken()
			if elem.CanSet() {
				elem.Set(reflect.Zero(elemType))
			}
		} else {
			if err := elemDec(dec, elem, o); err != nil {
				return err
			}
		}
		i++
	}
	if i < v.Len() {
		v.SetLen(i)
	}
	if v.IsNil() {
		v.Set(reflect.MakeSlice(v.Type(), 0, 0))
	}
	if _, err := dec.ReadToken(); err != nil {
		return err
	}
	return nil
}

func unmarshalFixedArray(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	if dec.PeekKind() != jsontext.BeginArray {
		return newSemanticError(v.Type(), "expected array")
	}
	dec.ReadToken()

	i := 0
	for dec.More() {
		if i >= v.Len() {
			return newSemanticError(v.Type(), "array length mismatch")
		}
		if err := unmarshalValue(dec, v.Index(i), o); err != nil {
			return err
		}
		i++
	}
	if _, err := dec.ReadToken(); err != nil {
		return err
	}
	if i != v.Len() {
		return newSemanticError(v.Type(), fmt.Sprintf("expected %d elements, got %d", v.Len(), i))
	}
	return nil
}

func unmarshalAnyValue(dec *jsontext.Decoder) (any, error) {
	k := dec.PeekKind()
	switch k {
	case jsontext.Null:
		dec.ReadToken()
		return nil, nil
	case jsontext.True:
		dec.ReadToken()
		return true, nil
	case jsontext.False:
		dec.ReadToken()
		return false, nil
	case jsontext.String:
		s, ok := dec.ReadStringFast()
		if ok {
			return s, nil
		}
		tok, err := dec.ReadToken()
		if err != nil {
			return nil, err
		}
		return tok.Str(), nil
	case jsontext.Number:
		raw, ok := dec.ReadNumberStr()
		if ok {
			return parseAnyNumber(raw)
		}
		tok, err := dec.ReadToken()
		if err != nil {
			return nil, err
		}
		return parseAnyNumber(tokenText(tok))
	case jsontext.BeginArray:
		dec.ReadToken()
		arr := make([]any, 0, 8)
		for dec.More() {
			elem, err := unmarshalAnyValue(dec)
			if err != nil {
				return nil, err
			}
			arr = append(arr, elem)
		}
		if _, err := dec.ReadToken(); err != nil {
			return nil, err
		}
		return arr, nil
	case jsontext.BeginObject:
		dec.ReadToken()
		m := make(map[string]any, 8)
		for dec.More() {
			key, ok := dec.ReadStringFast()
			if !ok {
				tok, err := dec.ReadToken()
				if err != nil {
					return nil, err
				}
				key = tok.Str()
			}
			val, err := unmarshalAnyValue(dec)
			if err != nil {
				return nil, err
			}
			m[key] = val
		}
		if _, err := dec.ReadToken(); err != nil {
			return nil, err
		}
		return m, nil
	default:
		return nil, fmt.Errorf("json: unexpected token kind")
	}
}

var (
	mapStringAnyType = reflect.TypeFor[map[string]any]()
	anyType          = reflect.TypeFor[any]()
)

func unmarshalMap(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	if dec.PeekKind() != jsontext.BeginObject {
		return newSemanticError(v.Type(), "expected object")
	}

	t := v.Type()

	if t == mapStringAnyType {
		dec.ReadToken()
		m := make(map[string]any, 8)
		for dec.More() {
			key, ok := dec.ReadStringFast()
			if !ok {
				tok, err := dec.ReadToken()
				if err != nil {
					return err
				}
				key = tok.Str()
			}
			val, err := unmarshalAnyValue(dec)
			if err != nil {
				return err
			}
			m[key] = val
		}
		if _, err := dec.ReadToken(); err != nil {
			return err
		}
		v.Set(reflect.ValueOf(m))
		return nil
	}

	dec.ReadToken()

	if v.IsNil() {
		v.Set(reflect.MakeMap(t))
	}
	keyType := t.Key()
	valType := t.Elem()
	isStringKey := keyType.Kind() == reflect.String

	var keyVal reflect.Value
	if isStringKey {
		keyVal = reflect.New(keyType).Elem()
	}

	for dec.More() {
		tok, err := dec.ReadToken()
		if err != nil {
			return err
		}
		if tok.Kind() != jsontext.String {
			return newSemanticError(t, "expected string key")
		}

		if isStringKey {
			keyVal.SetString(tok.Str())
		} else {
			keyVal = reflect.New(keyType).Elem()
			switch keyType.Kind() {
			case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
				n, err := parseIntValue(tok.Str(), keyType, false)
				if err != nil {
					return err
				}
				keyVal.SetInt(n)
			case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
				n, err := parseUintValue(tok.Str(), keyType, false)
				if err != nil {
					return err
				}
				keyVal.SetUint(n)
			default:
				if keyVal.CanAddr() {
					if u, ok := keyVal.Addr().Interface().(encoding.TextUnmarshaler); ok {
						if err := u.UnmarshalText([]byte(tok.Str())); err != nil {
							return err
						}
					}
				}
			}
		}

		val := reflect.New(valType).Elem()
		if err := unmarshalValue(dec, val, o); err != nil {
			return err
		}
		v.SetMapIndex(keyVal, val)
	}
	if _, err := dec.ReadToken(); err != nil {
		return err
	}
	return nil
}

func unmarshalStruct(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	if dec.PeekKind() != jsontext.BeginObject {
		return newSemanticError(v.Type(), "expected object")
	}
	dec.ReadToken()

	fs := cachedFields(v.Type())
	caseInsensitive := o.has(flagMatchCaseInsensitive)
	canUnsafe := v.CanAddr()
	var ptr unsafe.Pointer
	if canUnsafe {
		ptr = v.Addr().UnsafePointer()
	}

	for dec.More() {
		name, ok := dec.ReadStringFast()
		if !ok {
			tok, err := dec.ReadToken()
			if err != nil {
				return err
			}
			if tok.Kind() != jsontext.String {
				return newSemanticError(v.Type(), "expected string key")
			}
			name = tok.Str()
		}

		sf, found := fs.fieldByName(name, caseInsensitive)
		if !found && fs.fallback != nil {
			if err := unmarshalInlineMember(dec, v, fs.fallback, name, o); err != nil {
				return err
			}
			continue
		}
		if !found {
			if o.has(flagRejectUnknownMembers) {
				return &SemanticError{GoType: v.Type(), Msg: "unknown field: " + name, Err: ErrUnknownName}
			}
			if err := dec.SkipValue(); err != nil {
				return err
			}
			continue
		}

		pk := dec.PeekKind()

		if canUnsafe && sf.direct && sf.plainDecode {
			handled, err := decodeDirectField(dec, unsafe.Add(ptr, sf.offset), sf.typ, pk)
			if err != nil {
				return err
			}
			if handled {
				continue
			}
		}

		var fv reflect.Value
		if canUnsafe && sf.direct {
			fv = reflect.NewAt(sf.typ, unsafe.Add(ptr, sf.offset)).Elem()
		} else {
			fv = fieldByIndexAlloc(v, sf.index)
			if !fv.IsValid() || !fv.CanSet() {
				if err := dec.SkipValue(); err != nil {
					return err
				}
				continue
			}
		}
		if err := unmarshalValue(dec, fv, o); err != nil {
			return err
		}
	}
	if _, err := dec.ReadToken(); err != nil {
		return err
	}
	return nil
}

func unmarshalInlineMember(dec *jsontext.Decoder, v reflect.Value, fallback *inlineFallback, name string, o *Options) error {
	m := fieldByIndexAlloc(v, fallback.index)
	if !m.IsValid() || !m.CanSet() {
		return dec.SkipValue()
	}
	if m.IsNil() {
		m.Set(reflect.MakeMap(fallback.typ))
	}
	elem := reflect.New(fallback.typ.Elem()).Elem()
	if err := unmarshalValue(dec, elem, o); err != nil {
		return err
	}
	m.SetMapIndex(reflect.ValueOf(name).Convert(fallback.typ.Key()), elem)
	return nil
}

func decodeDirectField(dec *jsontext.Decoder, fp unsafe.Pointer, t reflect.Type, pk jsontext.Kind) (bool, error) {
	k := t.Kind()
	switch {
	case k == reflect.String && pk == jsontext.String:
		s, ok := dec.ReadStringFast()
		if !ok {
			return false, nil
		}
		*(*string)(fp) = s
		return true, nil
	case k >= reflect.Int && k <= reflect.Int64 && pk == jsontext.Number:
		raw, ok := dec.ReadNumberStr()
		if !ok {
			return false, nil
		}
		n, err := parseIntValue(raw, t, true)
		if err != nil {
			return true, err
		}
		storeInt(fp, k, n)
		return true, nil
	case k >= reflect.Uint && k <= reflect.Uintptr && pk == jsontext.Number:
		raw, ok := dec.ReadNumberStr()
		if !ok {
			return false, nil
		}
		n, err := parseUintValue(raw, t, true)
		if err != nil {
			return true, err
		}
		storeUint(fp, k, n)
		return true, nil
	case (k == reflect.Float64 || k == reflect.Float32) && pk == jsontext.Number:
		raw, ok := dec.ReadNumberStr()
		if !ok {
			return false, nil
		}
		f, err := strconv.ParseFloat(raw, t.Bits())
		if err != nil {
			return true, newSemanticError(t, "invalid float: "+raw)
		}
		if k == reflect.Float32 {
			*(*float32)(fp) = float32(f)
		} else {
			*(*float64)(fp) = f
		}
		return true, nil
	case k == reflect.Bool && (pk == jsontext.True || pk == jsontext.False):
		if _, err := dec.ReadToken(); err != nil {
			return true, err
		}
		*(*bool)(fp) = pk == jsontext.True
		return true, nil
	}
	return false, nil
}

func fieldByIndexAlloc(v reflect.Value, index []int) reflect.Value {
	for _, i := range index {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				if !v.CanSet() {
					return reflect.Value{}
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(i)
	}
	return v
}

func unmarshalInterface(dec *jsontext.Decoder, v reflect.Value, o *Options) error {
	if v.Type() == anyType {
		val, err := unmarshalAnyValue(dec)
		if err != nil {
			return err
		}
		if val == nil {
			v.Set(reflect.Zero(v.Type()))
		} else {
			v.Set(reflect.ValueOf(val))
		}
		return nil
	}

	val, err := unmarshalAnyValue(dec)
	if err != nil {
		return err
	}
	if val == nil {
		v.SetZero()
		return nil
	}
	rv := reflect.ValueOf(val)
	if !rv.Type().AssignableTo(v.Type()) {
		return newSemanticError(v.Type(), "cannot assign "+rv.Type().String())
	}
	v.Set(rv)
	return nil
}
