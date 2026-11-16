package json

import (
	"encoding"
	"encoding/base64"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unsafe"

	"github.com/guno1928/alos-json/internal/fnum"
	"github.com/guno1928/alos-json/jsontext"
)

const initialEncodeBufCap = 4096

type encodeFunc func(e *encodeState, p unsafe.Pointer) error

type typeEncoder struct {
	fn          encodeFunc
	directIface bool
	custom      bool
}

type encodeState struct {
	buf      []byte
	base     int
	opts     Options
	encoders *encoderCache
}

func (e *encodeState) appendInt(v int64) {
	e.buf = fnum.AppendInt64(e.buf, e.base, v)
}

func (e *encodeState) appendUint(v uint64) {
	e.buf = fnum.AppendUint64(e.buf, e.base, v)
}

var encodeStatePool = sync.Pool{New: func() any { return &encodeState{buf: make([]byte, 0, initialEncodeBufCap)} }}

type encoderCache struct {
	types      sync.Map
	mu         sync.Mutex
	marshalers *Marshalers
	flags      uint64
	dynamicAny bool
}

func newEncoderCache(m *Marshalers, flags uint64) *encoderCache {
	return &encoderCache{marshalers: m, flags: flags, dynamicAny: m != nil || flags&flagStringifyNumbers != 0}
}

func (c *encoderCache) encoderFor(t reflect.Type) *typeEncoder {
	if v, ok := c.types.Load(t); ok {
		return v.(*typeEncoder)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	b := &encoderBuilder{cache: c, building: make(map[reflect.Type]*typeEncoder)}
	te := b.build(t)
	for bt, bte := range b.building {
		c.types.Store(bt, bte)
	}
	return te
}

type encoderBuilder struct {
	cache    *encoderCache
	building map[reflect.Type]*typeEncoder
}

func (b *encoderBuilder) nativeOK() bool {
	return b.cache.marshalers == nil && b.cache.flags&flagStringifyNumbers == 0
}

func (b *encoderBuilder) has(flag uint64) bool {
	return b.cache.flags&flag != 0
}

func (b *encoderBuilder) build(t reflect.Type) *typeEncoder {
	if v, ok := b.cache.types.Load(t); ok {
		return v.(*typeEncoder)
	}
	if te, ok := b.building[t]; ok {
		return te
	}
	te := &typeEncoder{directIface: isDirectIface(t)}
	b.building[t] = te
	te.fn = makeEncodeFunc(t, b)
	if fns := b.cache.marshalers.applicable(t); len(fns) > 0 {
		te.fn = customEncodeFunc(t, fns, te.fn)
		te.custom = true
	}
	return te
}

func makeEncodeFunc(t reflect.Type, b *encoderBuilder) encodeFunc {
	fn := makeMethodOrKindEncodeFunc(t, b)
	if (t.Kind() == reflect.Pointer || t.Kind() == reflect.Interface) && hasCustomMarshaler(t) {
		return nilSafe(fn)
	}
	return fn
}

func nilSafe(fn encodeFunc) encodeFunc {
	return func(e *encodeState, p unsafe.Pointer) error {
		if *(*unsafe.Pointer)(p) == nil {
			e.buf = append(e.buf, "null"...)
			return nil
		}
		return fn(e, p)
	}
}

func makeMethodOrKindEncodeFunc(t reflect.Type, b *encoderBuilder) encodeFunc {
	pt := reflect.PointerTo(t)
	switch {
	case t.Implements(marshalerToType) || pt.Implements(marshalerToType):
		return encodeMarshalerTo(t)
	case t.Implements(marshalerType) || pt.Implements(marshalerType):
		return encodeMarshaler(t)
	case t.Implements(textMarshalerType) || pt.Implements(textMarshalerType):
		return encodeTextMarshaler(t)
	}
	fn := makeKindEncodeFunc(t, b)
	if _, scalar := scalarEncoders[t.Kind()]; scalar && t.Kind() != reflect.Bool && b.has(flagStringifyNumbers) {
		return quotedNumber(fn)
	}
	return fn
}

func quotedNumber(fn encodeFunc) encodeFunc {
	return func(e *encodeState, p unsafe.Pointer) error {
		e.buf = append(e.buf, '"')
		if err := fn(e, p); err != nil {
			return err
		}
		e.buf = append(e.buf, '"')
		return nil
	}
}

func makeKindEncodeFunc(t reflect.Type, b *encoderBuilder) encodeFunc {
	switch t.Kind() {
	case reflect.Bool:
		return encodeBool
	case reflect.Int:
		return encodeInt
	case reflect.Int8:
		return encodeInt8
	case reflect.Int16:
		return encodeInt16
	case reflect.Int32:
		return encodeInt32
	case reflect.Int64:
		return encodeInt64
	case reflect.Uint, reflect.Uintptr:
		return encodeUint
	case reflect.Uint8:
		return encodeUint8
	case reflect.Uint16:
		return encodeUint16
	case reflect.Uint32:
		return encodeUint32
	case reflect.Uint64:
		return encodeUint64
	case reflect.Float32:
		return encodeFloat32
	case reflect.Float64:
		return encodeFloat64
	case reflect.String:
		return encodeString
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return encodeBytes
		}
		if fn := integerSliceEncoder(t.Elem()); fn != nil && b.nativeOK() {
			return fn
		}
		return encodeSlice(t, b.build(t.Elem()))
	case reflect.Array:
		return encodeArray(t, b.build(t.Elem()))
	case reflect.Map:
		return encodeMap(t, b)
	case reflect.Struct:
		return encodeStruct(t, b)
	case reflect.Pointer:
		return encodePointer(b.build(t.Elem()))
	case reflect.Interface:
		if t.NumMethod() == 0 {
			return encodeInterface
		}
		return encodeNonEmptyInterface(t)
	default:
		return func(*encodeState, unsafe.Pointer) error { return newSemanticError(t, "unsupported type") }
	}
}

func (e *encodeState) appendString(s string) {
	if e.opts.has(flagEscapeHTML) {
		e.buf = jsontext.AppendQuoteStringHTML(e.buf, s)
		return
	}
	e.buf = appendJStr(e.buf, s)
}

func marshalFast(v any, opts []Option) ([]byte, error) {
	e := encodeStatePool.Get().(*encodeState)
	e.buf = e.buf[:0]
	e.base = 0
	e.opts = Options{}
	for _, fn := range opts {
		fn(&e.opts)
	}
	e.encoders = encodersFor(&e.opts)
	err := e.marshalTop(v)
	var out []byte
	switch {
	case err != nil:
	case e.opts.has(flagMultiline):
		out = appendIndented(make([]byte, 0, len(e.buf)*2), e.buf, e.opts.prefix, e.opts.indent)
	default:
		out = make([]byte, len(e.buf))
		copy(out, e.buf)
	}
	encodeStatePool.Put(e)
	return out, err
}

func marshalAppendFast(dst []byte, v any, opts []Option) ([]byte, error) {
	e := encodeStatePool.Get().(*encodeState)
	own := e.buf
	e.buf, e.base, e.opts = dst, len(dst), Options{}
	for _, fn := range opts {
		fn(&e.opts)
	}
	e.encoders = encodersFor(&e.opts)
	err := e.marshalTop(v)
	out := e.buf
	if err == nil && e.opts.has(flagMultiline) {
		own = appendIndented(own[:0], out[e.base:], e.opts.prefix, e.opts.indent)
		out = append(out[:e.base], own...)
	}
	e.buf = own[:0]
	encodeStatePool.Put(e)
	return out, err
}

func appendIndented(dst, src []byte, prefix, indent string) []byte {
	depth := 0
	pendingOpen := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if pendingOpen && c != '}' && c != ']' {
			pendingOpen = false
			depth++
			dst = appendIndentNewline(dst, prefix, indent, depth)
		}
		switch c {
		case '"':
			end := i + 1
			for ; src[end] != '"'; end++ {
				if src[end] == '\\' {
					end++
				}
			}
			dst = append(dst, src[i:end+1]...)
			i = end
		case '{', '[':
			pendingOpen = true
			dst = append(dst, c)
		case ',':
			dst = append(dst, c)
			dst = appendIndentNewline(dst, prefix, indent, depth)
		case ':':
			dst = append(dst, c, ' ')
		case '}', ']':
			if pendingOpen {
				pendingOpen = false
			} else {
				depth--
				dst = appendIndentNewline(dst, prefix, indent, depth)
			}
			dst = append(dst, c)
		default:
			dst = append(dst, c)
		}
	}
	return dst
}

func appendIndentNewline(dst []byte, prefix, indent string, depth int) []byte {
	dst = append(dst, '\n')
	dst = append(dst, prefix...)
	for range depth {
		dst = append(dst, indent...)
	}
	return dst
}

func (e *encodeState) marshalTop(v any) error {
	for {
		if v == nil {
			e.buf = append(e.buf, "null"...)
			return nil
		}
		t := reflect.TypeOf(v)
		if t.Kind() != reflect.Pointer {
			return e.anyValue(v)
		}
		rv := reflect.ValueOf(v)
		if rv.IsNil() {
			e.buf = append(e.buf, "null"...)
			return nil
		}
		elem := rv.Elem()
		if elem.Kind() != reflect.Interface && elem.Kind() != reflect.Pointer {
			return e.encoders.encoderFor(elem.Type()).fn(e, rv.UnsafePointer())
		}
		v = elem.Interface()
	}
}

type emptyInterface struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

func interfaceData(v any) unsafe.Pointer {
	return (*emptyInterface)(unsafe.Pointer(&v)).data
}

func (e *encodeState) anyValue(v any) error {
	if e.encoders.dynamicAny {
		return e.dynamicValue(v)
	}
	return e.plainAnyValue(v)
}

func (e *encodeState) plainAnyValue(v any) error {
	switch x := v.(type) {
	case nil:
		e.buf = append(e.buf, "null"...)
	case string:
		e.appendString(x)
	case float64:
		return appendFloat(&e.buf, x, 64)
	case bool:
		e.appendBool(x)
	case map[string]any:
		return e.mapStringAny(x)
	case []any:
		return e.anySlice(x)
	case int:
		e.appendInt(int64(x))
	case int64:
		e.appendInt(x)
	default:
		return e.dynamicValue(v)
	}
	return nil
}

func (e *encodeState) dynamicValue(v any) error {
	if v == nil {
		e.buf = append(e.buf, "null"...)
		return nil
	}
	te := e.encoders.encoderFor(reflect.TypeOf(v))
	if te.directIface {
		word := interfaceData(v)
		return te.fn(e, unsafe.Pointer(&word))
	}
	return te.fn(e, interfaceData(v))
}

func (e *encodeState) appendBool(b bool) {
	if b {
		e.buf = append(e.buf, "true"...)
	} else {
		e.buf = append(e.buf, "false"...)
	}
}

func encodeInterface(e *encodeState, p unsafe.Pointer) error {
	return e.anyValue(*(*any)(p))
}

func encodeNonEmptyInterface(t reflect.Type) encodeFunc {
	return func(e *encodeState, p unsafe.Pointer) error {
		v := reflect.NewAt(t, p).Elem()
		if v.IsNil() {
			e.buf = append(e.buf, "null"...)
			return nil
		}
		return e.anyValue(v.Elem().Interface())
	}
}

func encodeBool(e *encodeState, p unsafe.Pointer) error {
	e.appendBool(*(*bool)(p))
	return nil
}

func encodeInt(e *encodeState, p unsafe.Pointer) error {
	e.appendInt(int64(*(*int)(p)))
	return nil
}

func encodeInt8(e *encodeState, p unsafe.Pointer) error {
	e.appendInt(int64(*(*int8)(p)))
	return nil
}

func encodeInt16(e *encodeState, p unsafe.Pointer) error {
	e.appendInt(int64(*(*int16)(p)))
	return nil
}

func encodeInt32(e *encodeState, p unsafe.Pointer) error {
	e.appendInt(int64(*(*int32)(p)))
	return nil
}

func encodeInt64(e *encodeState, p unsafe.Pointer) error {
	e.appendInt(*(*int64)(p))
	return nil
}

func encodeUint(e *encodeState, p unsafe.Pointer) error {
	e.appendUint(uint64(*(*uint)(p)))
	return nil
}

func encodeUint8(e *encodeState, p unsafe.Pointer) error {
	e.appendUint(uint64(*(*uint8)(p)))
	return nil
}

func encodeUint16(e *encodeState, p unsafe.Pointer) error {
	e.appendUint(uint64(*(*uint16)(p)))
	return nil
}

func encodeUint32(e *encodeState, p unsafe.Pointer) error {
	e.appendUint(uint64(*(*uint32)(p)))
	return nil
}

func encodeUint64(e *encodeState, p unsafe.Pointer) error {
	e.appendUint(*(*uint64)(p))
	return nil
}

func encodeFloat32(e *encodeState, p unsafe.Pointer) error {
	return appendFloat(&e.buf, float64(*(*float32)(p)), 32)
}

func encodeFloat64(e *encodeState, p unsafe.Pointer) error {
	return appendFloat(&e.buf, *(*float64)(p), 64)
}

func encodeString(e *encodeState, p unsafe.Pointer) error {
	e.appendString(*(*string)(p))
	return nil
}

func encodeBytes(e *encodeState, p unsafe.Pointer) error {
	b := *(*[]byte)(p)
	if b == nil {
		e.buf = append(e.buf, "null"...)
		return nil
	}
	n := base64.StdEncoding.EncodedLen(len(b))
	e.buf = append(e.buf, '"')
	start := len(e.buf)
	e.buf = growSlice(e.buf, n)
	base64.StdEncoding.Encode(e.buf[start:start+n], b)
	e.buf = append(e.buf, '"')
	return nil
}

func (e *encodeState) nilSlice() {
	if e.opts.has(flagFormatNilSliceAsNull) {
		e.buf = append(e.buf, "null"...)
	} else {
		e.buf = append(e.buf, '[', ']')
	}
}

func encodeSlice(t reflect.Type, elem *typeEncoder) encodeFunc {
	size := t.Elem().Size()
	return func(e *encodeState, p unsafe.Pointer) error {
		hdr := (*sliceHeader)(p)
		if hdr.data == nil {
			e.nilSlice()
			return nil
		}
		return e.elements(hdr.data, hdr.len, size, elem)
	}
}

func integerSliceEncoder(elem reflect.Type) encodeFunc {
	if hasCustomMarshaler(elem) {
		return nil
	}
	switch elem.Kind() {
	case reflect.Int:
		return encodeSignedSlice[int]
	case reflect.Int8:
		return encodeSignedSlice[int8]
	case reflect.Int16:
		return encodeSignedSlice[int16]
	case reflect.Int32:
		return encodeSignedSlice[int32]
	case reflect.Int64:
		return encodeSignedSlice[int64]
	case reflect.Uint:
		return encodeUnsignedSlice[uint]
	case reflect.Uint16:
		return encodeUnsignedSlice[uint16]
	case reflect.Uint32:
		return encodeUnsignedSlice[uint32]
	case reflect.Uint64:
		return encodeUnsignedSlice[uint64]
	}
	return nil
}

func encodeSignedSlice[T int | int8 | int16 | int32 | int64](e *encodeState, p unsafe.Pointer) error {
	hdr := (*sliceHeader)(p)
	if hdr.data == nil {
		e.nilSlice()
		return nil
	}
	e.buf = append(e.buf, '[')
	for i, v := range unsafe.Slice((*T)(hdr.data), hdr.len) {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		e.appendInt(int64(v))
	}
	e.buf = append(e.buf, ']')
	return nil
}

func encodeUnsignedSlice[T uint | uint16 | uint32 | uint64](e *encodeState, p unsafe.Pointer) error {
	hdr := (*sliceHeader)(p)
	if hdr.data == nil {
		e.nilSlice()
		return nil
	}
	e.buf = append(e.buf, '[')
	for i, v := range unsafe.Slice((*T)(hdr.data), hdr.len) {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		e.appendUint(uint64(v))
	}
	e.buf = append(e.buf, ']')
	return nil
}

func encodeArray(t reflect.Type, elem *typeEncoder) encodeFunc {
	size, n := t.Elem().Size(), t.Len()
	return func(e *encodeState, p unsafe.Pointer) error {
		return e.elements(p, n, size, elem)
	}
}

func (e *encodeState) elements(data unsafe.Pointer, n int, size uintptr, elem *typeEncoder) error {
	e.buf = append(e.buf, '[')
	for i := range n {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		if err := elem.fn(e, unsafe.Add(data, uintptr(i)*size)); err != nil {
			return err
		}
	}
	e.buf = append(e.buf, ']')
	return nil
}

func (e *encodeState) anySlice(s []any) error {
	if s == nil {
		e.nilSlice()
		return nil
	}
	e.buf = append(e.buf, '[')
	for i, v := range s {
		if i > 0 {
			e.buf = append(e.buf, ',')
		}
		if err := e.plainAnyValue(v); err != nil {
			return err
		}
	}
	e.buf = append(e.buf, ']')
	return nil
}

func encodePointer(elem *typeEncoder) encodeFunc {
	return func(e *encodeState, p unsafe.Pointer) error {
		target := *(*unsafe.Pointer)(p)
		if target == nil {
			e.buf = append(e.buf, "null"...)
			return nil
		}
		return elem.fn(e, target)
	}
}

func (e *encodeState) nilMap() {
	if e.opts.has(flagFormatNilMapAsNull) {
		e.buf = append(e.buf, "null"...)
	} else {
		e.buf = append(e.buf, '{', '}')
	}
}

func (e *encodeState) mapStringAny(m map[string]any) error {
	if m == nil {
		e.nilMap()
		return nil
	}
	e.buf = append(e.buf, '{')
	first := true
	entry := func(k string, v any) error {
		if !first {
			e.buf = append(e.buf, ',')
		}
		first = false
		e.appendString(k)
		e.buf = append(e.buf, ':')
		return e.plainAnyValue(v)
	}
	if e.opts.has(flagDeterministic) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			if err := entry(k, m[k]); err != nil {
				return err
			}
		}
	} else {
		for k, v := range m {
			if err := entry(k, v); err != nil {
				return err
			}
		}
	}
	e.buf = append(e.buf, '}')
	return nil
}

func encodeMap(t reflect.Type, b *encoderBuilder) encodeFunc {
	if !b.nativeOK() {
		return encodeReflectMap(t, b)
	}
	if t.Key().Kind() == reflect.String && t.Elem() == anyType {
		return func(e *encodeState, p unsafe.Pointer) error {
			return e.mapStringAny(*(*map[string]any)(p))
		}
	}
	if t.Key() == stringType {
		switch t.Elem() {
		case stringType:
			return encodeNativeMap(func(e *encodeState, v string) error {
				e.appendString(v)
				return nil
			})
		case intType:
			return encodeNativeMap(func(e *encodeState, v int) error {
				e.appendInt(int64(v))
				return nil
			})
		}
	}
	return encodeReflectMap(t, b)
}

func encodeReflectMap(t reflect.Type, b *encoderBuilder) encodeFunc {
	key, customKeys := defaultMapKeyEncoder(t.Key()), false
	if fns := b.cache.marshalers.applicable(t.Key()); len(fns) > 0 {
		key, customKeys = customMapKeyEncoder(t.Key(), fns), true
	}
	members := mapMembersEncoder(t, b.build(t.Elem()), key, customKeys)
	return func(e *encodeState, p unsafe.Pointer) error {
		m := reflect.NewAt(t, p).Elem()
		if m.IsNil() {
			e.nilMap()
			return nil
		}
		e.buf = append(e.buf, '{')
		if err := members(e, m, true, nil); err != nil {
			return err
		}
		e.buf = append(e.buf, '}')
		return nil
	}
}

type mapKeyEncoder func(e *encodeState, key reflect.Value) error

func defaultMapKeyEncoder(keyType reflect.Type) mapKeyEncoder {
	if keyType.Kind() == reflect.String {
		return func(e *encodeState, key reflect.Value) error {
			e.appendString(key.String())
			return nil
		}
	}
	return func(e *encodeState, key reflect.Value) error {
		e.buf = appendMapKeyStr(e.buf, key)
		return nil
	}
}

func customMapKeyEncoder(keyType reflect.Type, fns []typedFunc[marshalFn]) mapKeyEncoder {
	fallback := defaultMapKeyEncoder(keyType)
	encode := customEncodeFunc(keyType, fns, func(e *encodeState, p unsafe.Pointer) error {
		return fallback(e, reflect.NewAt(keyType, p).Elem())
	})
	return func(e *encodeState, key reflect.Value) error {
		start := len(e.buf)
		if err := encode(e, key.Addr().UnsafePointer()); err != nil {
			return err
		}
		if len(e.buf) == start || e.buf[start] != '"' {
			return &SemanticError{GoType: keyType, Msg: "map key must encode as a JSON string"}
		}
		return nil
	}
}

func (key mapKeyEncoder) render(e *encodeState, keyVal reflect.Value) (raw []byte, name string, err error) {
	start := len(e.buf)
	if err := key(e, keyVal); err != nil {
		return nil, "", err
	}
	raw = slices.Clone(e.buf[start:])
	e.buf = e.buf[:start]
	err = Unmarshal(raw, &name)
	return raw, name, err
}

type mapMembersFunc func(e *encodeState, m reflect.Value, first bool, checkName func(string) error) error

func mapMembersEncoder(t reflect.Type, elem *typeEncoder, key mapKeyEncoder, customKeys bool) mapMembersFunc {
	keyType, elemType := t.Key(), t.Elem()
	return func(e *encodeState, m reflect.Value, first bool, checkName func(string) error) error {
		keyVal := reflect.New(keyType).Elem()
		elemVal := reflect.New(elemType).Elem()
		elemPtr := elemVal.Addr().UnsafePointer()
		entry := func(rawKey []byte) error {
			if checkName != nil {
				if err := checkName(keyVal.String()); err != nil {
					return err
				}
			}
			if !first {
				e.buf = append(e.buf, ',')
			}
			first = false
			if rawKey != nil {
				e.buf = append(e.buf, rawKey...)
			} else if err := key(e, keyVal); err != nil {
				return err
			}
			e.buf = append(e.buf, ':')
			return elem.fn(e, elemPtr)
		}
		if !e.opts.has(flagDeterministic) {
			iter := m.MapRange()
			for iter.Next() {
				keyVal.SetIterKey(iter)
				elemVal.SetIterValue(iter)
				if err := entry(nil); err != nil {
					return err
				}
			}
			return nil
		}
		keys := m.MapKeys()
		names := make([]string, len(keys))
		var rawKeys [][]byte
		if customKeys {
			rawKeys = make([][]byte, len(keys))
		}
		for i := range keys {
			if !customKeys {
				names[i] = mapKeyString(keys[i])
				continue
			}
			keyVal.Set(keys[i])
			var err error
			if rawKeys[i], names[i], err = key.render(e, keyVal); err != nil {
				return err
			}
		}
		order := make([]int, len(keys))
		for i := range order {
			order[i] = i
		}
		slices.SortStableFunc(order, func(a, b int) int { return strings.Compare(names[a], names[b]) })
		for _, idx := range order {
			keyVal.Set(keys[idx])
			elemVal.Set(m.MapIndex(keys[idx]))
			var rawKey []byte
			if customKeys {
				rawKey = rawKeys[idx]
			}
			if err := entry(rawKey); err != nil {
				return err
			}
		}
		return nil
	}
}

func encodeNativeMap[V any](appendValue func(e *encodeState, v V) error) encodeFunc {
	return func(e *encodeState, p unsafe.Pointer) error {
		m := *(*map[string]V)(p)
		if m == nil {
			e.nilMap()
			return nil
		}
		e.buf = append(e.buf, '{')
		first := true
		entry := func(k string, v V) error {
			if !first {
				e.buf = append(e.buf, ',')
			}
			first = false
			e.appendString(k)
			e.buf = append(e.buf, ':')
			return appendValue(e, v)
		}
		if e.opts.has(flagDeterministic) {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, k := range keys {
				if err := entry(k, m[k]); err != nil {
					return err
				}
			}
		} else {
			for k, v := range m {
				if err := entry(k, v); err != nil {
					return err
				}
			}
		}
		e.buf = append(e.buf, '}')
		return nil
	}
}

type fieldEncoder struct {
	key    string
	prefix string
	te     *typeEncoder
	typ    reflect.Type
	fieldLocation
	omitZero  bool
	omitEmpty bool
	asString  bool
}

type fallbackEncoder struct {
	*inlineFallback
	members        mapMembersFunc
	structType     reflect.Type
	structFields   *structFields
	fields         []fieldEncoder
	omitZeroFields bool
}

func appendFieldKey(name string) string {
	b := appendJStr(nil, name)
	return string(append(b, ':'))
}

func encodeStruct(t reflect.Type, b *encoderBuilder) encodeFunc {
	fs := cachedFields(t)
	fields := make([]fieldEncoder, len(fs.list))
	omitZeroFields := b.has(flagOmitZeroStructFields)
	simple := fs.fallback == nil && !omitZeroFields
	for i := range fs.list {
		sf := &fs.list[i]
		fields[i] = fieldEncoder{
			fieldLocation: sf.fieldLocation,
			key:           appendFieldKey(sf.name),
			te:            b.build(sf.typ),
			typ:           sf.typ,
			omitZero:      sf.omitZero,
			omitEmpty:     sf.omitEmpty,
			asString:      sf.asString,
		}
		if !sf.direct || sf.omitZero || sf.omitEmpty || sf.asString {
			simple = false
		}
	}
	var fallback *fallbackEncoder
	if fs.fallback != nil {
		fallback = &fallbackEncoder{
			inlineFallback: fs.fallback,
			members:        mapMembersEncoder(fs.fallback.typ, b.build(fs.fallback.typ.Elem()), defaultMapKeyEncoder(fs.fallback.typ.Key()), false),
			structType:     t,
			structFields:   fs,
			fields:         fields,
			omitZeroFields: omitZeroFields,
		}
	}
	general := func(e *encodeState, p unsafe.Pointer) error {
		e.buf = append(e.buf, '{')
		first := true
		for i := range fields {
			f := &fields[i]
			fp := unsafe.Add(p, f.offset)
			if !f.direct {
				var ok bool
				if fp, ok = f.existing(t, p); !ok {
					continue
				}
			}
			if f.omitZero || f.omitEmpty || omitZeroFields {
				fv := reflect.NewAt(f.typ, fp).Elem()
				if (f.omitZero || omitZeroFields) && isZero(fv) || f.omitEmpty && isEmpty(fv) {
					continue
				}
			}
			if !first {
				e.buf = append(e.buf, ',')
			}
			first = false
			e.buf = append(e.buf, f.key...)
			var err error
			if f.asString {
				err = e.quotedScalar(f.typ, f.te, fp)
			} else {
				err = f.te.fn(e, fp)
			}
			if err != nil {
				return err
			}
		}
		if fallback != nil {
			if err := e.inlineMembers(fallback, first, p); err != nil {
				return err
			}
		}
		e.buf = append(e.buf, '}')
		return nil
	}
	if !simple || len(fields) == 0 {
		return general
	}
	fields[0].prefix = "{" + fields[0].key
	for i := 1; i < len(fields); i++ {
		fields[i].prefix = "," + fields[i].key
	}
	return func(e *encodeState, p unsafe.Pointer) error {
		for i := range fields {
			f := &fields[i]
			e.buf = append(e.buf, f.prefix...)
			if err := f.te.fn(e, unsafe.Add(p, f.offset)); err != nil {
				return err
			}
		}
		e.buf = append(e.buf, '}')
		return nil
	}
}

func (f *fieldEncoder) omits(fv reflect.Value, omitZeroFields bool) bool {
	return (f.omitZero || omitZeroFields) && isZero(fv) || f.omitEmpty && isEmpty(fv)
}

func (f *fieldEncoder) written(t reflect.Type, p unsafe.Pointer, omitZeroFields bool) bool {
	fp, ok := f.existing(t, p)
	return ok && !f.omits(reflect.NewAt(f.typ, fp).Elem(), omitZeroFields)
}

func (e *encodeState) inlineMembers(fallback *fallbackEncoder, first bool, p unsafe.Pointer) error {
	t, fs, fields, omitZeroFields := fallback.structType, fallback.structFields, fallback.fields, fallback.omitZeroFields
	mp, ok := fallback.existing(t, p)
	if !ok {
		return nil
	}
	m := reflect.NewAt(fallback.typ, mp).Elem()
	if m.Len() == 0 {
		return nil
	}
	caseInsensitive := e.opts.has(flagMatchCaseInsensitive)
	return fallback.members(e, m, first, func(name string) error {
		if i := fs.indexOf(name, caseInsensitive); i >= 0 && fields[i].written(t, p, omitZeroFields) {
			return &SemanticError{GoType: t, Msg: "duplicate object member name " + strconv.Quote(name)}
		}
		return nil
	})
}

func (e *encodeState) quotedScalar(t reflect.Type, te *typeEncoder, p unsafe.Pointer) error {
	for !te.custom && t.Kind() == reflect.Pointer {
		p = *(*unsafe.Pointer)(p)
		if p == nil {
			e.buf = append(e.buf, "null"...)
			return nil
		}
		t = t.Elem()
		te = e.encoders.encoderFor(t)
	}
	scalar, ok := scalarEncoders[t.Kind()]
	if te.custom || !ok {
		return te.fn(e, p)
	}
	e.buf = append(e.buf, '"')
	if err := scalar(e, p); err != nil {
		return err
	}
	e.buf = append(e.buf, '"')
	return nil
}

var scalarEncoders = map[reflect.Kind]encodeFunc{
	reflect.Int: encodeInt, reflect.Int8: encodeInt8, reflect.Int16: encodeInt16, reflect.Int32: encodeInt32, reflect.Int64: encodeInt64,
	reflect.Uint: encodeUint, reflect.Uintptr: encodeUint, reflect.Uint8: encodeUint8, reflect.Uint16: encodeUint16, reflect.Uint32: encodeUint32, reflect.Uint64: encodeUint64,
	reflect.Float32: encodeFloat32, reflect.Float64: encodeFloat64, reflect.Bool: encodeBool,
}

func methodValue(t reflect.Type, p unsafe.Pointer, iface reflect.Type) any {
	v := reflect.NewAt(t, p)
	if !t.Implements(iface) {
		return v.Interface()
	}
	return v.Elem().Interface()
}

func encodeMarshalerTo(t reflect.Type) encodeFunc {
	return func(e *encodeState, p unsafe.Pointer) error {
		enc := jsontext.WrapBuffer(e.buf)
		err := methodValue(t, p, marshalerToType).(MarshalerTo).MarshalJSONTo(enc)
		e.buf = enc.Bytes()
		jsontext.PutEncoder(enc)
		return err
	}
}

func encodeMarshaler(t reflect.Type) encodeFunc {
	return func(e *encodeState, p unsafe.Pointer) error {
		b, err := methodValue(t, p, marshalerType).(Marshaler).MarshalJSON()
		if err != nil {
			return err
		}
		e.buf = append(e.buf, b...)
		return nil
	}
}

func encodeTextMarshaler(t reflect.Type) encodeFunc {
	return func(e *encodeState, p unsafe.Pointer) error {
		b, err := methodValue(t, p, textMarshalerType).(encoding.TextMarshaler).MarshalText()
		if err != nil {
			return err
		}
		e.appendString(b2s(b))
		return nil
	}
}
