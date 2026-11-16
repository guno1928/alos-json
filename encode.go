package json

import (
	"encoding"
	"encoding/base64"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/guno1928/alos-json/internal/fnum"
	"github.com/guno1928/alos-json/jsontext"
)

const (
	initialEncodeBufCap = 4096
	handoffStringLen    = 32 << 10
)

type encodeFunc func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error)

type typeEncoder struct {
	fn          encodeFunc
	rtype       unsafe.Pointer
	directIface bool
	custom      bool
}

type encodeState struct {
	buf        []byte
	opts       Options
	encoders   *encoderCache
	pooledData *byte
	handoff    *byte
}

func (e *encodeState) appendInt(b []byte, v int64) []byte {
	return fnum.AppendInt64(b, v)
}

func (e *encodeState) appendUint(b []byte, v uint64) []byte {
	return fnum.AppendUint64(b, v)
}

var encodeStatePool = sync.Pool{New: func() any { return &encodeState{buf: make([]byte, 0, initialEncodeBufCap)} }}

const typeSlotCount = 256

type encoderCache struct {
	slots      [typeSlotCount]atomic.Pointer[typeEncoder]
	types      sync.Map
	mu         sync.Mutex
	marshalers *Marshalers
	flags      uint64
	dynamicAny bool
}

func typePointer(t reflect.Type) unsafe.Pointer {
	return (*[2]unsafe.Pointer)(unsafe.Pointer(&t))[1]
}

func typeSlotIndex(rtype unsafe.Pointer) uintptr {
	return uintptr(rtype) >> 5 & (typeSlotCount - 1)
}

func newEncoderCache(m *Marshalers, flags uint64) *encoderCache {
	return &encoderCache{marshalers: m, flags: flags, dynamicAny: m != nil || flags&flagStringifyNumbers != 0}
}

func (c *encoderCache) encoderFor(t reflect.Type) *typeEncoder {
	rtype := typePointer(t)
	slot := &c.slots[typeSlotIndex(rtype)]
	if te := slot.Load(); te != nil && te.rtype == rtype {
		return te
	}
	te := c.loadOrBuild(t)
	slot.Store(te)
	return te
}

func (c *encoderCache) loadOrBuild(t reflect.Type) *typeEncoder {
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

func (b *encoderBuilder) plainPointer(t reflect.Type) bool {
	return t.Kind() == reflect.Pointer && !hasCustomMarshaler(t) && len(b.cache.marshalers.applicable(t)) == 0
}

func (b *encoderBuilder) build(t reflect.Type) *typeEncoder {
	if v, ok := b.cache.types.Load(t); ok {
		return v.(*typeEncoder)
	}
	if te, ok := b.building[t]; ok {
		return te
	}
	te := &typeEncoder{rtype: typePointer(t), directIface: isDirectIface(t)}
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
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		if *(*unsafe.Pointer)(p) == nil {
			return append(b, "null"...), nil
		}
		return fn(e, b, p)
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
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		b, err := fn(e, append(b, '"'), p)
		if err != nil {
			return b, err
		}
		return append(b, '"'), nil
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
		if fn := numberSliceEncoder(t.Elem()); fn != nil && b.nativeOK() {
			return fn
		}
		if t.Elem().Kind() == reflect.String && !hasCustomMarshaler(t.Elem()) && b.nativeOK() {
			return encodeStringSlice
		}
		if b.plainPointer(t.Elem()) {
			return encodePointerSlice(b.build(t.Elem().Elem()))
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
		return func(_ *encodeState, b []byte, _ unsafe.Pointer) ([]byte, error) {
			return b, newSemanticError(t, "unsupported type")
		}
	}
}

func (e *encodeState) appendString(b []byte, s string) []byte {
	return appendQuotedString(b, s, e.opts.has(flagEscapeHTML))
}

func (e *encodeState) appendLargeString(b []byte, s string) []byte {
	before := unsafe.SliceData(b)
	b = e.appendString(b, s)
	if after := unsafe.SliceData(b); before == e.pooledData && before != nil && after != before {
		e.handoff = after
	}
	return b
}

func (e *encodeState) canHandOff() bool {
	return e.handoff != nil && unsafe.SliceData(e.buf) == e.handoff
}

func marshalFast(v any, opts []Option) ([]byte, error) {
	e := encodeStatePool.Get().(*encodeState)
	e.buf = e.buf[:0]
	pooled := e.buf
	e.pooledData, e.handoff = unsafe.SliceData(pooled), nil
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
	case e.canHandOff():
		out = e.buf
		e.buf = pooled
	default:
		out = make([]byte, len(e.buf))
		copy(out, e.buf)
	}
	e.pooledData, e.handoff = nil, nil
	encodeStatePool.Put(e)
	return out, err
}

func marshalAppendFast(dst []byte, v any, opts []Option) ([]byte, error) {
	e := encodeStatePool.Get().(*encodeState)
	e.buf, e.opts = e.buf[:0], Options{}
	for _, fn := range opts {
		fn(&e.opts)
	}
	e.encoders = encodersFor(&e.opts)
	err := e.marshalTop(v)
	if err == nil && e.opts.has(flagMultiline) {
		dst = appendIndented(dst, e.buf, e.opts.prefix, e.opts.indent)
	} else {
		dst = append(dst, e.buf...)
	}
	encodeStatePool.Put(e)
	return dst, err
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
	b, err := e.encodeTop(e.buf, v)
	e.buf = b
	return err
}

func (e *encodeState) encodeTop(b []byte, v any) ([]byte, error) {
	for {
		if v == nil {
			return append(b, "null"...), nil
		}
		t := reflect.TypeOf(v)
		if t.Kind() != reflect.Pointer {
			return e.anyValue(b, v)
		}
		p := interfaceData(v)
		if p == nil {
			return append(b, "null"...), nil
		}
		elem := t.Elem()
		if k := elem.Kind(); k != reflect.Interface && k != reflect.Pointer {
			return e.encoders.encoderFor(elem).fn(e, b, p)
		}
		v = reflect.NewAt(elem, p).Elem().Interface()
	}
}

type emptyInterface struct {
	typ  unsafe.Pointer
	data unsafe.Pointer
}

func interfaceData(v any) unsafe.Pointer {
	return (*emptyInterface)(unsafe.Pointer(&v)).data
}

func (e *encodeState) anyValue(b []byte, v any) ([]byte, error) {
	if e.encoders.dynamicAny {
		return e.dynamicValue(b, v)
	}
	return e.plainAnyValue(b, v)
}

func (e *encodeState) plainAnyValue(b []byte, v any) ([]byte, error) {
	switch x := v.(type) {
	case nil:
		return append(b, "null"...), nil
	case string:
		return e.appendString(b, x), nil
	case float64:
		return appendFloat64(b, x)
	case bool:
		return appendBool(b, x), nil
	case map[string]any:
		return e.mapStringAny(b, x)
	case []any:
		return e.anySlice(b, x)
	case int:
		return e.appendInt(b, int64(x)), nil
	case int64:
		return e.appendInt(b, x), nil
	default:
		return e.dynamicValue(b, v)
	}
}

func (e *encodeState) dynamicValue(b []byte, v any) ([]byte, error) {
	if v == nil {
		return append(b, "null"...), nil
	}
	te := e.encoders.encoderFor(reflect.TypeOf(v))
	if te.directIface {
		word := interfaceData(v)
		return te.fn(e, b, unsafe.Pointer(&word))
	}
	return te.fn(e, b, interfaceData(v))
}

func appendBool(b []byte, v bool) []byte {
	if v {
		return append(b, "true"...)
	}
	return append(b, "false"...)
}

func encodeInterface(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.anyValue(b, *(*any)(p))
}

func encodeNonEmptyInterface(t reflect.Type) encodeFunc {
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		v := reflect.NewAt(t, p).Elem()
		if v.IsNil() {
			return append(b, "null"...), nil
		}
		return e.anyValue(b, v.Elem().Interface())
	}
}

func encodeBool(_ *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return appendBool(b, *(*bool)(p)), nil
}

func encodeInt(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.appendInt(b, int64(*(*int)(p))), nil
}

func encodeInt8(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.appendInt(b, int64(*(*int8)(p))), nil
}

func encodeInt16(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.appendInt(b, int64(*(*int16)(p))), nil
}

func encodeInt32(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.appendInt(b, int64(*(*int32)(p))), nil
}

func encodeInt64(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.appendInt(b, *(*int64)(p)), nil
}

func encodeUint(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.appendUint(b, uint64(*(*uint)(p))), nil
}

func encodeUint8(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.appendUint(b, uint64(*(*uint8)(p))), nil
}

func encodeUint16(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.appendUint(b, uint64(*(*uint16)(p))), nil
}

func encodeUint32(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.appendUint(b, uint64(*(*uint32)(p))), nil
}

func encodeUint64(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return e.appendUint(b, *(*uint64)(p)), nil
}

func encodeFloat32(_ *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	err := appendFloat(&b, float64(*(*float32)(p)), 32)
	return b, err
}

func encodeFloat64(_ *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	return appendFloat64(b, *(*float64)(p))
}

func encodeString(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	s := *(*string)(p)
	if len(s) >= handoffStringLen {
		return e.appendLargeString(b, s), nil
	}
	return e.appendString(b, s), nil
}

func encodeBytes(_ *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	src := *(*[]byte)(p)
	if src == nil {
		return append(b, "null"...), nil
	}
	n := base64.StdEncoding.EncodedLen(len(src))
	b = append(b, '"')
	start := len(b)
	b = growSlice(b, n)
	base64.StdEncoding.Encode(b[start:start+n], src)
	return append(b, '"'), nil
}

func (e *encodeState) nilSlice(b []byte) []byte {
	if e.opts.has(flagFormatNilSliceAsNull) {
		return append(b, "null"...)
	}
	return append(b, '[', ']')
}

func encodeSlice(t reflect.Type, elem *typeEncoder) encodeFunc {
	size := t.Elem().Size()
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		hdr := (*sliceHeader)(p)
		if hdr.data == nil {
			return e.nilSlice(b), nil
		}
		return e.elements(b, hdr.data, hdr.len, size, elem)
	}
}

func encodePointerSlice(target *typeEncoder) encodeFunc {
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		hdr := (*sliceHeader)(p)
		if hdr.data == nil {
			return e.nilSlice(b), nil
		}
		b = append(b, '[')
		var err error
		for i, ptr := range unsafe.Slice((*unsafe.Pointer)(hdr.data), hdr.len) {
			if i > 0 {
				b = append(b, ',')
			}
			if ptr == nil {
				b = append(b, "null"...)
			} else if b, err = target.fn(e, b, ptr); err != nil {
				return b, err
			}
		}
		return append(b, ']'), nil
	}
}

func encodeStringSlice(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	hdr := (*sliceHeader)(p)
	if hdr.data == nil {
		return e.nilSlice(b), nil
	}
	b = append(b, '[')
	for i, s := range unsafe.Slice((*string)(hdr.data), hdr.len) {
		if i > 0 {
			b = append(b, ',')
		}
		if len(s) >= handoffStringLen {
			b = e.appendLargeString(b, s)
		} else {
			b = e.appendString(b, s)
		}
	}
	return append(b, ']'), nil
}

func numberSliceEncoder(elem reflect.Type) encodeFunc {
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
	case reflect.Float64:
		return encodeFloat64Slice
	}
	return nil
}

func encodeSignedSlice[T int | int8 | int16 | int32 | int64](e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	hdr := (*sliceHeader)(p)
	if hdr.data == nil {
		return e.nilSlice(b), nil
	}
	b = append(b, '[')
	if fnum.IntListKernelUseful(hdr.len, unsafe.Sizeof(T(0))) {
		return closeList(fnum.AppendIntListKernel(b, unsafe.Slice((*T)(hdr.data), hdr.len)), hdr.len), nil
	}
	for _, v := range unsafe.Slice((*T)(hdr.data), hdr.len) {
		u := uint64(v)
		if v < 0 {
			b = append(b, '-')
			u = -u
		}
		b = append(fnum.AppendUint64(b, u), ',')
	}
	return closeList(b, hdr.len), nil
}

func encodeUnsignedSlice[T uint | uint16 | uint32 | uint64](e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	hdr := (*sliceHeader)(p)
	if hdr.data == nil {
		return e.nilSlice(b), nil
	}
	b = append(b, '[')
	if fnum.IntListKernelUseful(hdr.len, unsafe.Sizeof(T(0))) {
		return closeList(fnum.AppendUintListKernel(b, unsafe.Slice((*T)(hdr.data), hdr.len)), hdr.len), nil
	}
	for _, v := range unsafe.Slice((*T)(hdr.data), hdr.len) {
		b = append(fnum.AppendUint64(b, uint64(v)), ',')
	}
	return closeList(b, hdr.len), nil
}

func encodeFloat64Slice(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
	hdr := (*sliceHeader)(p)
	if hdr.data == nil {
		return e.nilSlice(b), nil
	}
	b = append(b, '[')
	for _, f := range unsafe.Slice((*float64)(hdr.data), hdr.len) {
		b = slices.Grow(b, fnum.FloatWindow)
		w := floatWindow(b)
		n := fnum.PutFloat64(w, f)
		if n == 0 {
			if err := appendFloat(&b, f, 64); err != nil {
				return b, err
			}
			b = append(b, ',')
			continue
		}
		w[n] = ','
		b = b[:len(b)+n+1]
	}
	return closeList(b, hdr.len), nil
}

func closeList(b []byte, n int) []byte {
	if n == 0 {
		return append(b, ']')
	}
	b[len(b)-1] = ']'
	return b
}

func encodeArray(t reflect.Type, elem *typeEncoder) encodeFunc {
	size, n := t.Elem().Size(), t.Len()
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		return e.elements(b, p, n, size, elem)
	}
}

func (e *encodeState) elements(b []byte, data unsafe.Pointer, n int, size uintptr, elem *typeEncoder) ([]byte, error) {
	b = append(b, '[')
	for i := range n {
		if i > 0 {
			b = append(b, ',')
		}
		var err error
		if b, err = elem.fn(e, b, unsafe.Add(data, uintptr(i)*size)); err != nil {
			return b, err
		}
	}
	return append(b, ']'), nil
}

func (e *encodeState) anySlice(b []byte, s []any) ([]byte, error) {
	if s == nil {
		return e.nilSlice(b), nil
	}
	b = append(b, '[')
	for i, v := range s {
		if i > 0 {
			b = append(b, ',')
		}
		var err error
		if b, err = e.plainAnyValue(b, v); err != nil {
			return b, err
		}
	}
	return append(b, ']'), nil
}

func encodePointer(elem *typeEncoder) encodeFunc {
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		target := *(*unsafe.Pointer)(p)
		if target == nil {
			return append(b, "null"...), nil
		}
		return elem.fn(e, b, target)
	}
}

func (e *encodeState) nilMap(b []byte) []byte {
	if e.opts.has(flagFormatNilMapAsNull) {
		return append(b, "null"...)
	}
	return append(b, '{', '}')
}

func (e *encodeState) mapStringAny(b []byte, m map[string]any) ([]byte, error) {
	if m == nil {
		return e.nilMap(b), nil
	}
	b = append(b, '{')
	first := true
	entry := func(b []byte, k string, v any) ([]byte, error) {
		if !first {
			b = append(b, ',')
		}
		first = false
		b = append(e.appendString(b, k), ':')
		return e.plainAnyValue(b, v)
	}
	var err error
	if e.opts.has(flagDeterministic) {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		for _, k := range keys {
			if b, err = entry(b, k, m[k]); err != nil {
				return b, err
			}
		}
	} else {
		for k, v := range m {
			if b, err = entry(b, k, v); err != nil {
				return b, err
			}
		}
	}
	return append(b, '}'), nil
}

func encodeMap(t reflect.Type, b *encoderBuilder) encodeFunc {
	if !b.nativeOK() {
		return encodeReflectMap(t, b)
	}
	if t.Key().Kind() == reflect.String && t.Elem() == anyType {
		return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
			return e.mapStringAny(b, *(*map[string]any)(p))
		}
	}
	if t.Key() == stringType {
		switch t.Elem() {
		case stringType:
			return encodeNativeMap(func(e *encodeState, b []byte, v string) ([]byte, error) {
				return e.appendString(b, v), nil
			})
		case intType:
			return encodeNativeMap(func(e *encodeState, b []byte, v int) ([]byte, error) {
				return e.appendInt(b, int64(v)), nil
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
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		m := reflect.NewAt(t, p).Elem()
		if m.IsNil() {
			return e.nilMap(b), nil
		}
		b, err := members(e, append(b, '{'), m, true, nil)
		if err != nil {
			return b, err
		}
		return append(b, '}'), nil
	}
}

type mapKeyEncoder func(e *encodeState, b []byte, key reflect.Value) ([]byte, error)

func defaultMapKeyEncoder(keyType reflect.Type) mapKeyEncoder {
	if keyType.Kind() == reflect.String {
		return func(e *encodeState, b []byte, key reflect.Value) ([]byte, error) {
			return e.appendString(b, key.String()), nil
		}
	}
	return func(_ *encodeState, b []byte, key reflect.Value) ([]byte, error) {
		return appendMapKeyStr(b, key), nil
	}
}

func customMapKeyEncoder(keyType reflect.Type, fns []typedFunc[marshalFn]) mapKeyEncoder {
	fallback := defaultMapKeyEncoder(keyType)
	encode := customEncodeFunc(keyType, fns, func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		return fallback(e, b, reflect.NewAt(keyType, p).Elem())
	})
	return func(e *encodeState, b []byte, key reflect.Value) ([]byte, error) {
		start := len(b)
		b, err := encode(e, b, key.Addr().UnsafePointer())
		if err != nil {
			return b, err
		}
		if len(b) == start || b[start] != '"' {
			return b, &SemanticError{GoType: keyType, Msg: "map key must encode as a JSON string"}
		}
		return b, nil
	}
}

func (key mapKeyEncoder) render(e *encodeState, keyVal reflect.Value) (raw []byte, name string, err error) {
	if raw, err = key(e, nil, keyVal); err != nil {
		return nil, "", err
	}
	err = Unmarshal(raw, &name)
	return raw, name, err
}

type mapMembersFunc func(e *encodeState, b []byte, m reflect.Value, first bool, checkName func(string) error) ([]byte, error)

func mapMembersEncoder(t reflect.Type, elem *typeEncoder, key mapKeyEncoder, customKeys bool) mapMembersFunc {
	keyType, elemType := t.Key(), t.Elem()
	return func(e *encodeState, b []byte, m reflect.Value, first bool, checkName func(string) error) ([]byte, error) {
		keyVal := reflect.New(keyType).Elem()
		elemVal := reflect.New(elemType).Elem()
		elemPtr := elemVal.Addr().UnsafePointer()
		entry := func(b, rawKey []byte) ([]byte, error) {
			if checkName != nil {
				if err := checkName(keyVal.String()); err != nil {
					return b, err
				}
			}
			if !first {
				b = append(b, ',')
			}
			first = false
			if rawKey != nil {
				b = append(b, rawKey...)
			} else {
				var err error
				if b, err = key(e, b, keyVal); err != nil {
					return b, err
				}
			}
			return elem.fn(e, append(b, ':'), elemPtr)
		}
		var err error
		if !e.opts.has(flagDeterministic) {
			iter := m.MapRange()
			for iter.Next() {
				keyVal.SetIterKey(iter)
				elemVal.SetIterValue(iter)
				if b, err = entry(b, nil); err != nil {
					return b, err
				}
			}
			return b, nil
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
			if rawKeys[i], names[i], err = key.render(e, keyVal); err != nil {
				return b, err
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
			if b, err = entry(b, rawKey); err != nil {
				return b, err
			}
		}
		return b, nil
	}
}

func encodeNativeMap[V any](appendValue func(e *encodeState, b []byte, v V) ([]byte, error)) encodeFunc {
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		m := *(*map[string]V)(p)
		if m == nil {
			return e.nilMap(b), nil
		}
		b = append(b, '{')
		first := true
		entry := func(b []byte, k string, v V) ([]byte, error) {
			if !first {
				b = append(b, ',')
			}
			first = false
			b = append(e.appendString(b, k), ':')
			return appendValue(e, b, v)
		}
		var err error
		if e.opts.has(flagDeterministic) {
			keys := make([]string, 0, len(m))
			for k := range m {
				keys = append(keys, k)
			}
			slices.Sort(keys)
			for _, k := range keys {
				if b, err = entry(b, k, m[k]); err != nil {
					return b, err
				}
			}
		} else {
			for k, v := range m {
				if b, err = entry(b, k, v); err != nil {
					return b, err
				}
			}
		}
		return append(b, '}'), nil
	}
}

const fieldPrefixPadLen = 32

type fieldEncoder struct {
	prefixPad [fieldPrefixPadLen]byte
	key       string
	prefix    string
	te        *typeEncoder
	typ       reflect.Type
	fieldLocation
	omitZero    bool
	omitEmpty   bool
	asString    bool
	conditional bool
	zeroCheck   omitCheck
	emptyCheck  omitCheck
}

type omitCheck uint8

const (
	omitCheckReflect omitCheck = iota
	omitCheckNilWord
	omitCheckEmptyString
	omitCheckEmptySlice
	omitCheckNilSlice
)

var isZeroerType = reflect.TypeFor[IsZeroer]()

func zeroCheckFor(t reflect.Type) omitCheck {
	if reflect.PointerTo(t).Implements(isZeroerType) {
		return omitCheckReflect
	}
	switch t.Kind() {
	case reflect.Pointer, reflect.Interface, reflect.Map:
		return omitCheckNilWord
	case reflect.String:
		return omitCheckEmptyString
	case reflect.Slice:
		return omitCheckNilSlice
	}
	return omitCheckReflect
}

func emptyCheckFor(t reflect.Type) omitCheck {
	switch t.Kind() {
	case reflect.Pointer, reflect.Interface:
		return omitCheckNilWord
	case reflect.String:
		return omitCheckEmptyString
	case reflect.Slice:
		return omitCheckEmptySlice
	}
	return omitCheckReflect
}

func (c omitCheck) holds(t reflect.Type, fp unsafe.Pointer, reflectCheck func(reflect.Value) bool) bool {
	switch c {
	case omitCheckNilWord:
		return *(*unsafe.Pointer)(fp) == nil
	case omitCheckEmptyString:
		return len(*(*string)(fp)) == 0
	case omitCheckEmptySlice:
		return (*sliceHeader)(fp).len == 0
	case omitCheckNilSlice:
		return (*sliceHeader)(fp).data == nil
	}
	return reflectCheck(reflect.NewAt(t, fp).Elem())
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
			conditional:   !sf.direct || sf.omitZero || sf.omitEmpty || omitZeroFields,
			zeroCheck:     zeroCheckFor(sf.typ),
			emptyCheck:    emptyCheckFor(sf.typ),
		}
		fields[i].setPrefix("," + fields[i].key)
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
	general := func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		b = append(b, '{')
		skipComma := 1
		var err error
		for i := range fields {
			f := &fields[i]
			fp := unsafe.Add(p, f.offset)
			if f.conditional {
				var ok bool
				if fp, ok = f.present(t, p, omitZeroFields); !ok {
					continue
				}
			}
			if skipComma == 0 {
				b = f.appendPrefix(b)
			} else {
				b = append(b, f.key...)
				skipComma = 0
			}
			if f.asString {
				b, err = e.quotedScalar(b, f.typ, f.te, fp)
			} else {
				b, err = f.te.fn(e, b, fp)
			}
			if err != nil {
				return b, err
			}
		}
		if fallback != nil {
			if b, err = e.inlineMembers(b, fallback, skipComma == 1, p); err != nil {
				return b, err
			}
		}
		return append(b, '}'), nil
	}
	if !simple || len(fields) == 0 {
		return general
	}
	fields[0].setPrefix("{" + fields[0].key)
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		var err error
		for i := range fields {
			f := &fields[i]
			if b, err = f.te.fn(e, f.appendPrefix(b), unsafe.Add(p, f.offset)); err != nil {
				return b, err
			}
		}
		return append(b, '}'), nil
	}
}

func (f *fieldEncoder) setPrefix(prefix string) {
	f.prefix = prefix
	copy(f.prefixPad[:], prefix)
}

func (f *fieldEncoder) appendPrefix(b []byte) []byte {
	if n := len(f.prefix); n < fieldPrefixPadLen && cap(b)-len(b) >= fieldPrefixPadLen {
		*(*[fieldPrefixPadLen]byte)(b[len(b) : len(b)+fieldPrefixPadLen]) = f.prefixPad
		return b[:len(b)+n]
	}
	return append(b, f.prefix...)
}

func (f *fieldEncoder) present(t reflect.Type, p unsafe.Pointer, omitZeroFields bool) (unsafe.Pointer, bool) {
	fp := unsafe.Add(p, f.offset)
	if !f.direct {
		var ok bool
		if fp, ok = f.existing(t, p); !ok {
			return nil, false
		}
	}
	if (f.omitZero || omitZeroFields) && f.zeroCheck.holds(f.typ, fp, isZero) || f.omitEmpty && f.emptyCheck.holds(f.typ, fp, isEmpty) {
		return nil, false
	}
	return fp, true
}

func (f *fieldEncoder) written(t reflect.Type, p unsafe.Pointer, omitZeroFields bool) bool {
	_, ok := f.present(t, p, omitZeroFields)
	return ok
}

func (e *encodeState) inlineMembers(b []byte, fallback *fallbackEncoder, first bool, p unsafe.Pointer) ([]byte, error) {
	t, fs, fields, omitZeroFields := fallback.structType, fallback.structFields, fallback.fields, fallback.omitZeroFields
	mp, ok := fallback.existing(t, p)
	if !ok {
		return b, nil
	}
	m := reflect.NewAt(fallback.typ, mp).Elem()
	if m.Len() == 0 {
		return b, nil
	}
	caseInsensitive := e.opts.has(flagMatchCaseInsensitive)
	return fallback.members(e, b, m, first, func(name string) error {
		if i := fs.indexOf(name, caseInsensitive); i >= 0 && fields[i].written(t, p, omitZeroFields) {
			return &SemanticError{GoType: t, Msg: "duplicate object member name " + strconv.Quote(name)}
		}
		return nil
	})
}

func (e *encodeState) quotedScalar(b []byte, t reflect.Type, te *typeEncoder, p unsafe.Pointer) ([]byte, error) {
	for !te.custom && t.Kind() == reflect.Pointer {
		p = *(*unsafe.Pointer)(p)
		if p == nil {
			return append(b, "null"...), nil
		}
		t = t.Elem()
		te = e.encoders.encoderFor(t)
	}
	scalar, ok := scalarEncoders[t.Kind()]
	if te.custom || !ok {
		return te.fn(e, b, p)
	}
	b, err := scalar(e, append(b, '"'), p)
	if err != nil {
		return b, err
	}
	return append(b, '"'), nil
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
	return func(_ *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		enc := jsontext.WrapBuffer(b)
		err := methodValue(t, p, marshalerToType).(MarshalerTo).MarshalJSONTo(enc)
		b = enc.Bytes()
		jsontext.PutEncoder(enc)
		return b, err
	}
}

func encodeMarshaler(t reflect.Type) encodeFunc {
	return func(_ *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		out, err := methodValue(t, p, marshalerType).(Marshaler).MarshalJSON()
		if err != nil {
			return b, err
		}
		return append(b, out...), nil
	}
}

func encodeTextMarshaler(t reflect.Type) encodeFunc {
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		out, err := methodValue(t, p, textMarshalerType).(encoding.TextMarshaler).MarshalText()
		if err != nil {
			return b, err
		}
		return e.appendString(b, b2s(out)), nil
	}
}
