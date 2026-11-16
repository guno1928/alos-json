package json

import (
	"encoding"
	"encoding/base64"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/guno1928/alos-json/internal/fnum"
	"github.com/guno1928/alos-json/internal/scan"
	"github.com/guno1928/alos-json/jsontext"
)

const (
	stringArenaChunk     = 4096
	stringArenaMaxInline = 1024
	initialSliceCap      = 4
)

type decodeFunc func(d *decodeState, p unsafe.Pointer) error

type typeDecoder struct {
	fn     decodeFunc
	zero   func(p unsafe.Pointer)
	custom bool
}

type decodeState struct {
	buf        []byte
	pos        int
	depth      int
	opts       Options
	arena      []byte
	keyBuf     []byte
	strBuf     []byte
	anyStack   []any
	floatSlab  []float64
	stringSlab []string
	sliceSlab  [][]any

	pointerSlabs []pointerSlab
}

var decodeStatePool = sync.Pool{New: func() any { return new(decodeState) }}

type decoderCache struct {
	types        sync.Map
	mu           sync.Mutex
	unmarshalers *Unmarshalers
}

var defaultDecoders decoderCache

func (c *decoderCache) decoderFor(t reflect.Type) *typeDecoder {
	if v, ok := c.types.Load(t); ok {
		return v.(*typeDecoder)
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	b := &decoderBuilder{cache: c, building: make(map[reflect.Type]*typeDecoder)}
	td := b.build(t)
	for bt, btd := range b.building {
		c.types.Store(bt, btd)
	}
	return td
}

type decoderBuilder struct {
	cache    *decoderCache
	building map[reflect.Type]*typeDecoder
}

func (b *decoderBuilder) customized() bool {
	return b.cache.unmarshalers != nil
}

func (b *decoderBuilder) build(t reflect.Type) *typeDecoder {
	if v, ok := b.cache.types.Load(t); ok {
		return v.(*typeDecoder)
	}
	if td, ok := b.building[t]; ok {
		return td
	}
	td := &typeDecoder{zero: zeroFuncFor(t)}
	b.building[t] = td
	td.fn = makeDecodeFunc(t, b)
	if fns := b.cache.unmarshalers.applicable(t); len(fns) > 0 {
		td.fn = customDecodeFunc(t, fns, td)
		td.custom = true
	}
	return td
}

func makeDecodeFunc(t reflect.Type, b *decoderBuilder) decodeFunc {
	pt := reflect.PointerTo(t)
	switch {
	case pt.Implements(unmarshalerFromType):
		return decodeUnmarshalerFrom(t)
	case pt.Implements(unmarshalerType):
		return decodeUnmarshaler(t)
	case pt.Implements(textUnmarshalerType):
		return decodeTextUnmarshaler(t, makeKindDecodeFunc(t, b))
	}
	return makeKindDecodeFunc(t, b)
}

func makeKindDecodeFunc(t reflect.Type, b *decoderBuilder) decodeFunc {
	switch t.Kind() {
	case reflect.Bool:
		return decodeBool(t)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return decodeInt(t)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return decodeUint(t)
	case reflect.Float32:
		return decodeFloat32(t)
	case reflect.Float64:
		return decodeFloat64(t)
	case reflect.String:
		return decodeString(t)
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return decodeBytes(t)
		}
		generic := decodeSlice(t, b.build(t.Elem()))
		if b.customized() {
			return generic
		}
		if fast := scalarSliceDecoder(t.Elem(), generic); fast != nil {
			return fast
		}
		return generic
	case reflect.Array:
		return decodeArray(t, b.build(t.Elem()))
	case reflect.Map:
		return decodeMap(t, b)
	case reflect.Struct:
		return decodeStruct(t, b)
	case reflect.Pointer:
		return decodePointer(t, b.build(t.Elem()))
	case reflect.Interface:
		if b.customized() {
			return decodeInterfaceWith(t, b.dynamicValue())
		}
		if t == anyType {
			return decodeEmptyInterface
		}
		return decodeInterfaceWith(t, (*decodeState).anyValue)
	default:
		return decodeSkip
	}
}

func hasPointers(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.String, reflect.Interface,
		reflect.Chan, reflect.Func, reflect.UnsafePointer:
		return true
	case reflect.Array:
		return t.Len() > 0 && hasPointers(t.Elem())
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			if hasPointers(t.Field(i).Type) {
				return true
			}
		}
	}
	return false
}

func zeroFuncFor(t reflect.Type) func(p unsafe.Pointer) {
	size := t.Size()
	switch {
	case size == 0:
		return func(unsafe.Pointer) {}
	case !hasPointers(t):
		return func(p unsafe.Pointer) { clear(unsafe.Slice((*byte)(p), size)) }
	default:
		return func(p unsafe.Pointer) { reflect.NewAt(t, p).Elem().SetZero() }
	}
}

func unmarshalBytesFast(data []byte, v any, opts []Option) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &SemanticError{Msg: "unmarshal requires non-nil pointer"}
	}
	d := decodeStatePool.Get().(*decodeState)
	d.buf = data
	d.pos = 0
	d.depth = 0
	d.opts = Options{}
	for _, fn := range opts {
		fn(&d.opts)
	}
	td := decodersFor(d.opts.unmarshalers).decoderFor(rv.Type().Elem())
	err := d.decodeTopLevel(td, rv.UnsafePointer())
	d.release()
	return err
}

func (d *decodeState) release() {
	d.buf = nil
	clear(d.anyStack[:cap(d.anyStack)])
	d.anyStack = d.anyStack[:0]
	decodeStatePool.Put(d)
}

func (d *decodeState) decodeTopLevel(td *typeDecoder, p unsafe.Pointer) error {
	d.skipSpace()
	if d.pos >= len(d.buf) {
		return d.syntaxError("unexpected end of input")
	}
	if err := d.value(td, p); err != nil {
		return err
	}
	d.skipSpace()
	if d.pos != len(d.buf) {
		return d.syntaxError("invalid data after top-level value")
	}
	return nil
}

func (d *decodeState) syntaxError(msg string) error {
	return &jsontext.SyntacticError{ByteOffset: int64(d.pos), Msg: msg}
}

func (d *decodeState) syntaxErrorAt(pos int, msg string) error {
	return &jsontext.SyntacticError{ByteOffset: int64(pos), Msg: msg}
}

func (d *decodeState) skipSpace() {
	buf, pos := d.buf, d.pos
	for pos < len(buf) && scan.IsSpace(buf[pos]) {
		pos++
	}
	d.pos = pos
}

func (d *decodeState) nextValueStart() error {
	d.skipSpace()
	if d.pos >= len(d.buf) {
		return d.syntaxError("unexpected end of input")
	}
	return nil
}

func (d *decodeState) value(td *typeDecoder, p unsafe.Pointer) error {
	if d.buf[d.pos] == 'n' && !td.custom {
		if err := d.literal("null"); err != nil {
			return err
		}
		td.zero(p)
		return nil
	}
	return td.fn(d, p)
}

func (d *decodeState) literal(lit string) error {
	end := d.pos + len(lit)
	if end > len(d.buf) || string(d.buf[d.pos:end]) != lit {
		return d.syntaxError("invalid literal")
	}
	d.pos = end
	return nil
}

func (d *decodeState) enter() error {
	d.depth++
	if d.depth > scan.MaxDepth {
		return d.syntaxError("exceeded max nesting depth")
	}
	d.pos++
	return nil
}

func (d *decodeState) afterElement(closer byte) (bool, error) {
	d.skipSpace()
	if d.pos >= len(d.buf) {
		return false, d.syntaxError("unexpected end of input")
	}
	switch d.buf[d.pos] {
	case ',':
		d.pos++
		return true, d.nextValueStart()
	case closer:
		d.pos++
		d.depth--
		return false, nil
	default:
		return false, d.syntaxError("missing ',' between elements")
	}
}

func (d *decodeState) skipValue() error {
	end, ok := scan.ValueEnd(d.buf, d.pos, d.depth)
	if !ok {
		return d.syntaxErrorAt(end, "invalid JSON value")
	}
	d.pos = end
	return nil
}

func (d *decodeState) unquoteInto(scratch []byte, start, special int) ([]byte, error) {
	out, end, ok := scan.AppendUnquoted(append(scratch[:0], d.buf[start:special]...), d.buf, special)
	if !ok {
		return out, d.syntaxErrorAt(end, "invalid string")
	}
	d.pos = end + 1
	return out, nil
}

func (d *decodeState) plainStringEnd() (start, special int, plain bool) {
	start = d.pos + 1
	special = scan.NextSpecial(d.buf, start)
	return start, special, special < len(d.buf) && d.buf[special] == '"'
}

func (d *decodeState) arenaReserve(n int) {
	if len(d.arena)+n > cap(d.arena) {
		d.arena = make([]byte, 0, stringArenaChunk)
	}
}

func (d *decodeState) ownString(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b) > stringArenaMaxInline {
		return string(b)
	}
	d.arenaReserve(len(b))
	start := len(d.arena)
	d.arena = append(d.arena, b...)
	return unsafe.String(&d.arena[start], len(b))
}

func (d *decodeState) readString() (string, error) {
	start, special, plain := d.plainStringEnd()
	if plain {
		d.pos = special + 1
		return d.ownString(d.buf[start:special]), nil
	}
	out, err := d.unquoteInto(d.strBuf, start, special)
	if err != nil {
		d.strBuf = out
		return "", err
	}
	if len(out) > stringArenaMaxInline {
		d.strBuf = nil
		return b2s(out), nil
	}
	d.strBuf = out
	return d.ownString(out), nil
}

func (d *decodeState) readStringView() ([]byte, error) {
	start, special, plain := d.plainStringEnd()
	if plain {
		d.pos = special + 1
		return d.buf[start:special], nil
	}
	out, err := d.unquoteInto(d.keyBuf, start, special)
	d.keyBuf = out
	return out, err
}

func (d *decodeState) numberText() (string, error) {
	end, ok := scan.NumberEnd(d.buf, d.pos)
	if !ok {
		return "", d.syntaxErrorAt(end, "invalid number")
	}
	s := b2s(d.buf[d.pos:end])
	d.pos = end
	return s, nil
}

func isNumberStart(c byte) bool { return c == '-' || c-'0' < 10 }

func kindError(t reflect.Type, want string, c byte) error {
	return newSemanticError(t, "expected "+want+", got "+jsontext.Kind(kindOfByte(c)).String())
}

func kindOfByte(c byte) byte {
	switch {
	case c == 'n' || c == 't' || c == 'f' || c == '"' || c == '{' || c == '}' || c == '[' || c == ']':
		return c
	case isNumberStart(c):
		return '0'
	}
	return 0
}

func decodeBool(t reflect.Type) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		switch d.buf[d.pos] {
		case 't':
			if err := d.literal("true"); err != nil {
				return err
			}
			*(*bool)(p) = true
		case 'f':
			if err := d.literal("false"); err != nil {
				return err
			}
			*(*bool)(p) = false
		case '"':
			switch rest := d.buf[d.pos:]; {
			case hasPrefix(rest, quotedTrue):
				d.pos += len(quotedTrue)
				*(*bool)(p) = true
			case hasPrefix(rest, quotedFalse):
				d.pos += len(quotedFalse)
				*(*bool)(p) = false
			default:
				return newSemanticError(t, "expected bool")
			}
		default:
			return newSemanticError(t, "expected bool")
		}
		return nil
	}
}

const (
	quotedTrue  = `"true"`
	quotedFalse = `"false"`
)

const maxFastIntDigits = 18

func (d *decodeState) fastInt() (int64, bool) {
	n, end, ok := parseFastInt(d.buf, d.pos)
	if ok {
		d.pos = end
	}
	return n, ok
}

func parseFastInt(b []byte, i int) (int64, int, bool) {
	neg := b[i] == '-'
	if neg {
		i++
	}
	if i >= len(b) {
		return 0, i, false
	}
	var n uint64
	switch c := b[i]; {
	case c == '0':
		i++
	case c >= '1' && c <= '9':
		start := i
		for i < len(b) && b[i]-'0' < 10 {
			if i-start == maxFastIntDigits {
				return 0, i, false
			}
			n = n*10 + uint64(b[i]-'0')
			i++
		}
	default:
		return 0, i, false
	}
	if i < len(b) {
		if c := b[i]; c == '.' || c == 'e' || c == 'E' || c-'0' < 10 {
			return 0, i, false
		}
	}
	if neg {
		return -int64(n), i, true
	}
	return int64(n), i, true
}

func (d *decodeState) quotedNumberText() (string, error) {
	b, err := d.readStringView()
	if err != nil {
		return "", err
	}
	return b2s(b), nil
}

func decodeInt(t reflect.Type) decodeFunc {
	kind, bits := t.Kind(), t.Bits()
	return func(d *decodeState, p unsafe.Pointer) error {
		c := d.buf[d.pos]
		if c == '"' {
			s, err := d.quotedNumberText()
			if err != nil {
				return err
			}
			n, err := parseIntValue(s, t, false)
			if err != nil {
				return err
			}
			storeInt(p, kind, n)
			return nil
		}
		if !isNumberStart(c) {
			return kindError(t, "number", c)
		}
		start := d.pos
		if n, ok := d.fastInt(); ok {
			if !intFits(n, bits) {
				return overflowError(t, b2s(d.buf[start:d.pos]))
			}
			storeInt(p, kind, n)
			return nil
		}
		s, err := d.numberText()
		if err != nil {
			return err
		}
		n, err := parseIntValue(s, t, true)
		if err != nil {
			return err
		}
		storeInt(p, kind, n)
		return nil
	}
}

func decodeUint(t reflect.Type) decodeFunc {
	kind := t.Kind()
	return func(d *decodeState, p unsafe.Pointer) error {
		c := d.buf[d.pos]
		var s string
		var err error
		switch {
		case c == '"':
			if s, err = d.quotedNumberText(); err != nil {
				return err
			}
			n, err := parseUintValue(s, t, false)
			if err != nil {
				return err
			}
			storeUint(p, kind, n)
			return nil
		case !isNumberStart(c):
			return kindError(t, "number", c)
		}
		if s, err = d.numberText(); err != nil {
			return err
		}
		n, err := parseUintValue(s, t, true)
		if err != nil {
			return err
		}
		storeUint(p, kind, n)
		return nil
	}
}

func (d *decodeState) floatText(t reflect.Type) (text string, quoted bool, err error) {
	c := d.buf[d.pos]
	if c == '"' {
		text, err = d.quotedNumberText()
		return text, true, err
	}
	if !isNumberStart(c) {
		return "", false, kindError(t, "number", c)
	}
	text, err = d.numberText()
	return text, false, err
}

func floatError(t reflect.Type, s string, quoted bool) error {
	if quoted {
		return newSemanticError(t, "cannot parse string as float: "+s)
	}
	return newSemanticError(t, "invalid float: "+s)
}

func decodeFloat64(t reflect.Type) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		s, quoted, err := d.floatText(t)
		if err != nil {
			return err
		}
		if !quoted {
			if f, ok := fnum.ParseFloat64(s); ok {
				*(*float64)(p) = f
				return nil
			}
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return floatError(t, s, quoted)
		}
		*(*float64)(p) = f
		return nil
	}
}

func decodeFloat32(t reflect.Type) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		s, quoted, err := d.floatText(t)
		if err != nil {
			return err
		}
		f, err := strconv.ParseFloat(s, 32)
		if err != nil {
			return floatError(t, s, quoted)
		}
		*(*float32)(p) = float32(f)
		return nil
	}
}

func decodeString(t reflect.Type) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		if d.buf[d.pos] != '"' {
			return newSemanticError(t, "expected string")
		}
		s, err := d.readString()
		if err != nil {
			return err
		}
		*(*string)(p) = s
		return nil
	}
}

func decodeBytes(t reflect.Type) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		if d.buf[d.pos] != '"' {
			return newSemanticError(t, "expected base64 string")
		}
		src, err := d.readStringView()
		if err != nil {
			return err
		}
		out := make([]byte, base64.StdEncoding.DecodedLen(len(src)))
		n, err := base64.StdEncoding.Decode(out, src)
		if err != nil {
			return newSemanticError(t, "invalid base64: "+err.Error())
		}
		*(*[]byte)(p) = out[:n]
		return nil
	}
}

type sliceHeader struct {
	data unsafe.Pointer
	len  int
	cap  int
}

func decodeSlice(t reflect.Type, elem *typeDecoder) decodeFunc {
	elemType := t.Elem()
	elemSize := elemType.Size()
	emptyData := reflect.MakeSlice(t, 0, 0).UnsafePointer()
	chunk := newSliceChunker(elemType)
	return func(d *decodeState, p unsafe.Pointer) error {
		if d.buf[d.pos] != '[' {
			return newSemanticError(t, "expected array")
		}
		if err := d.enter(); err != nil {
			return err
		}
		hdr := (*sliceHeader)(p)
		if err := d.nextValueStart(); err != nil {
			return err
		}
		if d.buf[d.pos] == ']' {
			d.pos++
			d.depth--
			hdr.len = 0
			if hdr.data == nil {
				hdr.data = emptyData
			}
			return nil
		}
		oldLen, reusedCap := hdr.len, hdr.cap
		n := 0
		for {
			if n == hdr.cap {
				if n == 0 && chunk.enabled {
					hdr.data, hdr.cap = d.allocPointer(chunk.id, chunk.arrayType, chunk.bytes, chunk.count), initialSliceCap
				} else {
					growSliceValue(t, p, n, max(initialSliceCap, hdr.cap*2))
				}
			}
			ep := unsafe.Add(hdr.data, uintptr(n)*elemSize)
			if n >= oldLen && n < reusedCap {
				elem.zero(ep)
			}
			n++
			hdr.len = n
			if err := d.value(elem, ep); err != nil {
				return err
			}
			more, err := d.afterElement(']')
			if err != nil {
				return err
			}
			if !more {
				return nil
			}
		}
	}
}

func scalarSliceDecoder(elem reflect.Type, generic decodeFunc) decodeFunc {
	if hasCustomUnmarshaler(elem) {
		return nil
	}
	switch elem.Kind() {
	case reflect.Int:
		return signedSliceDecoder[int](generic)
	case reflect.Int8:
		return signedSliceDecoder[int8](generic)
	case reflect.Int16:
		return signedSliceDecoder[int16](generic)
	case reflect.Int32:
		return signedSliceDecoder[int32](generic)
	case reflect.Int64:
		return signedSliceDecoder[int64](generic)
	case reflect.Float64:
		return float64SliceDecoder(generic)
	case reflect.String:
		return stringSliceDecoder(generic)
	}
	return nil
}

type signedInt interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

func signedSliceDecoder[T signedInt](generic decodeFunc) decodeFunc {
	bits := int(unsafe.Sizeof(T(0))) * 8
	return func(d *decodeState, p unsafe.Pointer) error {
		hdr := (*sliceHeader)(p)
		if d.buf[d.pos] == '[' && fillSignedSlice[T](d, hdr, bits) {
			return nil
		}
		return generic(d, p)
	}
}

func fastSliceStart(d *decodeState) (int, bool) {
	i := scan.SkipSpace(d.buf, d.pos+1)
	return i, d.depth < scan.MaxDepth && i < len(d.buf) && d.buf[i] != ']'
}

func fastSliceSeparator(buf []byte, end int) (next int, closed, ok bool) {
	i := scan.SkipSpace(buf, end)
	if i >= len(buf) {
		return i, false, false
	}
	switch buf[i] {
	case ',':
		i = scan.SkipSpace(buf, i+1)
		return i, false, i < len(buf)
	case ']':
		return i + 1, true, true
	}
	return i, false, false
}

func tightComma(buf []byte, end int) (int, bool) {
	if end+1 < len(buf) && buf[end] == ',' && buf[end+1] > ' ' {
		return end + 1, true
	}
	return end, false
}

func storeSliceElem[T any](s []T, n int, v T) []T {
	if n == len(s) {
		grown := make([]T, max(initialSliceCap, 2*len(s)))
		copy(grown, s)
		s = grown
	}
	s[n] = v
	return s
}

func commitFastSlice[T any](d *decodeState, hdr *sliceHeader, s []T, n, next int) {
	*hdr = sliceHeader{data: unsafe.Pointer(unsafe.SliceData(s)), len: n, cap: len(s)}
	d.pos = next
}

func fillSignedSlice[T signedInt](d *decodeState, hdr *sliceHeader, bits int) bool {
	i, ok := fastSliceStart(d)
	if !ok {
		return false
	}
	s := unsafe.Slice((*T)(hdr.data), hdr.cap)
	for n := 1; ; n++ {
		v, end, ok := parseFastInt(d.buf, i)
		if !ok || !intFits(v, bits) {
			return false
		}
		s = storeSliceElem(s, n-1, T(v))
		if next, ok := tightComma(d.buf, end); ok {
			i = next
			continue
		}
		next, closed, ok := fastSliceSeparator(d.buf, end)
		if !ok {
			return false
		}
		if closed {
			commitFastSlice(d, hdr, s, n, next)
			return true
		}
		i = next
	}
}

func float64SliceDecoder(generic decodeFunc) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		hdr := (*sliceHeader)(p)
		if d.buf[d.pos] == '[' && fillFloat64Slice(d, hdr) {
			return nil
		}
		return generic(d, p)
	}
}

func parseFastFloat64(b []byte, i int) (float64, int, bool) {
	if !isNumberStart(b[i]) {
		return 0, i, false
	}
	end, ok := scan.NumberEnd(b, i)
	if !ok {
		return 0, end, false
	}
	text := b2s(b[i:end])
	if f, ok := fnum.ParseFloat64(text); ok {
		return f, end, true
	}
	f, err := strconv.ParseFloat(text, 64)
	return f, end, err == nil
}

func fillFloat64Slice(d *decodeState, hdr *sliceHeader) bool {
	i, ok := fastSliceStart(d)
	if !ok {
		return false
	}
	s := unsafe.Slice((*float64)(hdr.data), hdr.cap)
	for n := 1; ; n++ {
		v, end, ok := parseFastFloat64(d.buf, i)
		if !ok {
			return false
		}
		s = storeSliceElem(s, n-1, v)
		if next, ok := tightComma(d.buf, end); ok {
			i = next
			continue
		}
		next, closed, ok := fastSliceSeparator(d.buf, end)
		if !ok {
			return false
		}
		if closed {
			commitFastSlice(d, hdr, s, n, next)
			return true
		}
		i = next
	}
}

func stringSliceDecoder(generic decodeFunc) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		hdr := (*sliceHeader)(p)
		if d.buf[d.pos] == '[' && fillStringSlice(d, hdr) {
			return nil
		}
		return generic(d, p)
	}
}

func fillStringSlice(d *decodeState, hdr *sliceHeader) bool {
	i, ok := fastSliceStart(d)
	if !ok {
		return false
	}
	start := d.pos
	s := unsafe.Slice((*string)(hdr.data), hdr.cap)
	for n := 1; ; n++ {
		if d.buf[i] != '"' {
			d.pos = start
			return false
		}
		d.pos = i
		v, err := d.readString()
		if err != nil {
			d.pos = start
			return false
		}
		s = storeSliceElem(s, n-1, v)
		if next, ok := tightComma(d.buf, d.pos); ok {
			i = next
			continue
		}
		next, closed, ok := fastSliceSeparator(d.buf, d.pos)
		if !ok {
			d.pos = start
			return false
		}
		if closed {
			commitFastSlice(d, hdr, s, n, next)
			return true
		}
		i = next
	}
}

type sliceChunker struct {
	enabled   bool
	id        int
	arrayType reflect.Type
	bytes     uintptr
	count     int
}

func newSliceChunker(elem reflect.Type) sliceChunker {
	bytes := elem.Size() * initialSliceCap
	if bytes == 0 || pointerSlabLen(bytes) < pointerSlabMinElems {
		return sliceChunker{}
	}
	count := pointerSlabLen(bytes)
	return sliceChunker{
		enabled:   true,
		id:        int(pointerSlabIDs.Add(1) - 1),
		arrayType: reflect.ArrayOf(count*initialSliceCap, elem),
		bytes:     bytes,
		count:     count,
	}
}

func growSliceValue(t reflect.Type, p unsafe.Pointer, n, newCap int) {
	grown := reflect.MakeSlice(t, n, newCap)
	if n > 0 {
		reflect.Copy(grown, reflect.NewAt(t, p).Elem())
	}
	*(*sliceHeader)(p) = sliceHeader{data: grown.UnsafePointer(), len: n, cap: newCap}
}

func decodeArray(t reflect.Type, elem *typeDecoder) decodeFunc {
	length := t.Len()
	elemSize := t.Elem().Size()
	return func(d *decodeState, p unsafe.Pointer) error {
		if d.buf[d.pos] != '[' {
			return newSemanticError(t, "expected array")
		}
		if err := d.enter(); err != nil {
			return err
		}
		if err := d.nextValueStart(); err != nil {
			return err
		}
		n := 0
		if d.buf[d.pos] == ']' {
			d.pos++
			d.depth--
		} else {
			for {
				if n >= length {
					return newSemanticError(t, "array length mismatch")
				}
				if err := d.value(elem, unsafe.Add(p, uintptr(n)*elemSize)); err != nil {
					return err
				}
				n++
				more, err := d.afterElement(']')
				if err != nil {
					return err
				}
				if !more {
					break
				}
			}
		}
		if n != length {
			return newSemanticError(t, "expected "+strconv.Itoa(length)+" elements, got "+strconv.Itoa(n))
		}
		return nil
	}
}

const (
	pointerSlabBytes    = 2048
	pointerSlabMaxElems = 32
	pointerSlabMinElems = 4
)

type pointerSlab struct {
	next unsafe.Pointer
	left int
}

var pointerSlabIDs atomic.Int32

func pointerSlabLen(size uintptr) int {
	return min(pointerSlabMaxElems, int(pointerSlabBytes/size))
}

func decodePointer(t reflect.Type, elem *typeDecoder) decodeFunc {
	elemType := t.Elem()
	size := elemType.Size()
	if size == 0 || pointerSlabLen(size) < pointerSlabMinElems {
		return func(d *decodeState, p unsafe.Pointer) error {
			target := *(*unsafe.Pointer)(p)
			if target == nil {
				target = reflect.New(elemType).UnsafePointer()
				*(*unsafe.Pointer)(p) = target
			}
			return d.value(elem, target)
		}
	}
	n := pointerSlabLen(size)
	arrayType := reflect.ArrayOf(n, elemType)
	id := int(pointerSlabIDs.Add(1) - 1)
	return func(d *decodeState, p unsafe.Pointer) error {
		target := *(*unsafe.Pointer)(p)
		if target == nil {
			target = d.allocPointer(id, arrayType, size, n)
			*(*unsafe.Pointer)(p) = target
		}
		return d.value(elem, target)
	}
}

func (d *decodeState) allocPointer(id int, arrayType reflect.Type, size uintptr, n int) unsafe.Pointer {
	if id >= len(d.pointerSlabs) {
		d.pointerSlabs = append(d.pointerSlabs, make([]pointerSlab, id+1-len(d.pointerSlabs))...)
	}
	s := &d.pointerSlabs[id]
	if s.left == 0 {
		s.next = reflect.New(arrayType).UnsafePointer()
		s.left = n
	}
	p := s.next
	s.left--
	if s.left > 0 {
		s.next = unsafe.Add(p, size)
	} else {
		s.next = nil
	}
	return p
}

type fieldDecoder struct {
	name   string
	quoted string
	td     *typeDecoder
	fieldLocation
}

func decodeStruct(t reflect.Type, b *decoderBuilder) decodeFunc {
	fs := cachedFields(t)
	fields := make([]fieldDecoder, len(fs.list))
	for i := range fs.list {
		sf := &fs.list[i]
		fields[i] = fieldDecoder{
			fieldLocation: sf.fieldLocation,
			name:          sf.name,
			quoted:        literalQuotedName(sf.name),
			td:            b.build(sf.typ),
		}
	}
	if fs.fallback != nil {
		b.build(fs.fallback.typ.Elem())
	}
	return func(d *decodeState, p unsafe.Pointer) error {
		if d.buf[d.pos] != '{' {
			return newSemanticError(t, "expected object")
		}
		if err := d.enter(); err != nil {
			return err
		}
		if err := d.nextValueStart(); err != nil {
			return err
		}
		if d.buf[d.pos] == '}' {
			d.pos++
			d.depth--
			return nil
		}
		next := 0
		for {
			var f *fieldDecoder
			var key []byte
			if next < len(fields) && hasPrefix(d.buf[d.pos:], fields[next].quoted) {
				f = &fields[next]
				next++
				d.pos += len(f.quoted)
				if err := d.colon(); err != nil {
					return err
				}
			} else {
				var err error
				if key, err = d.objectKey(); err != nil {
					return err
				}
				f = matchField(fields, fs, key, &next, &d.opts)
			}
			var err error
			switch {
			case f == nil:
				err = d.unknownMember(t, p, fs, key)
			case f.direct:
				err = d.value(f.td, unsafe.Add(p, f.offset))
			default:
				err = d.embeddedValue(t, p, f)
			}
			if err != nil {
				return err
			}
			more, err := d.afterElement('}')
			if err != nil {
				return err
			}
			if !more {
				return nil
			}
		}
	}
}

func (d *decodeState) objectKey() ([]byte, error) {
	if d.buf[d.pos] != '"' {
		return nil, d.syntaxError("expected string for object name")
	}
	key, err := d.readStringView()
	if err != nil {
		return nil, err
	}
	return key, d.colon()
}

func (d *decodeState) colon() error {
	d.skipSpace()
	if d.pos >= len(d.buf) || d.buf[d.pos] != ':' {
		return d.syntaxError("missing ':' after object name")
	}
	d.pos++
	return d.nextValueStart()
}

func literalQuotedName(name string) string {
	for i := 0; i < len(name); i++ {
		if c := name[i]; c < 0x20 || c == '"' || c == '\\' {
			return ""
		}
	}
	return `"` + name + `"`
}

func hasPrefix(b []byte, prefix string) bool {
	return prefix != "" && len(b) >= len(prefix) && string(b[:len(prefix)]) == prefix
}

func matchField(fields []fieldDecoder, fs *structFields, key []byte, next *int, o *Options) *fieldDecoder {
	if i := *next; i < len(fields) && fields[i].name == string(key) {
		*next = i + 1
		return &fields[i]
	}
	if i := fs.indexOf(string(key), o.has(flagMatchCaseInsensitive)); i >= 0 {
		*next = i + 1
		return &fields[i]
	}
	return nil
}

func (d *decodeState) embeddedValue(t reflect.Type, p unsafe.Pointer, f *fieldDecoder) error {
	fp, ok := f.allocated(t, p)
	if !ok {
		return d.skipValue()
	}
	return d.value(f.td, fp)
}

func (d *decodeState) unknownMember(t reflect.Type, p unsafe.Pointer, fs *structFields, key []byte) error {
	if fs.fallback != nil {
		return d.inlineMember(t, p, fs.fallback, key)
	}
	if d.opts.has(flagRejectUnknownMembers) {
		return &SemanticError{GoType: t, Msg: "unknown field: " + string(key), Err: ErrUnknownName}
	}
	return d.skipValue()
}

func (d *decodeState) inlineMember(t reflect.Type, p unsafe.Pointer, fallback *inlineFallback, key []byte) error {
	mp, ok := fallback.allocated(t, p)
	if !ok {
		return d.skipValue()
	}
	m := reflect.NewAt(fallback.typ, mp).Elem()
	if m.IsNil() {
		m.Set(reflect.MakeMap(fallback.typ))
	}
	keyVal := reflect.New(fallback.typ.Key()).Elem()
	keyVal.SetString(d.ownString(key))
	elemVal := reflect.New(fallback.typ.Elem()).Elem()
	elem := decodersFor(d.opts.unmarshalers).decoderFor(fallback.typ.Elem())
	if err := d.value(elem, elemVal.Addr().UnsafePointer()); err != nil {
		return err
	}
	m.SetMapIndex(keyVal, elemVal)
	return nil
}

var (
	stringType   = reflect.TypeFor[string]()
	intType      = reflect.TypeFor[int]()
	boolType     = reflect.TypeFor[bool]()
	sliceAnyType = reflect.TypeFor[[]any]()
)

func decodeMap(t reflect.Type, b *decoderBuilder) decodeFunc {
	if t == mapStringAnyType && !b.customized() {
		return decodeMapStringAny
	}
	if t.Key() == stringType && !b.customized() {
		switch t.Elem() {
		case stringType:
			return decodeNativeMap[string](t, b.build(stringType))
		case intType:
			return decodeNativeMap[int](t, b.build(intType))
		}
	}
	keyType, elemType := t.Key(), t.Elem()
	elem := b.build(elemType)
	customKey := customMapKeyDecoder(t, b)
	return func(d *decodeState, p unsafe.Pointer) error {
		if d.buf[d.pos] != '{' {
			return newSemanticError(t, "expected object")
		}
		if err := d.enter(); err != nil {
			return err
		}
		m := reflect.NewAt(t, p).Elem()
		if m.IsNil() {
			m.Set(reflect.MakeMap(t))
		}
		if err := d.nextValueStart(); err != nil {
			return err
		}
		if d.buf[d.pos] == '}' {
			d.pos++
			d.depth--
			return nil
		}
		keyVal := reflect.New(keyType).Elem()
		elemVal := reflect.New(elemType).Elem()
		elemPtr := elemVal.Addr().UnsafePointer()
		for {
			if customKey != nil {
				if err := customKey(d, keyVal); err != nil {
					return err
				}
			} else {
				key, err := d.objectKey()
				if err != nil {
					return err
				}
				if err := setMapKey(keyVal, keyType, t, key, d); err != nil {
					return err
				}
			}
			elem.zero(elemPtr)
			if err := d.value(elem, elemPtr); err != nil {
				return err
			}
			m.SetMapIndex(keyVal, elemVal)
			more, err := d.afterElement('}')
			if err != nil {
				return err
			}
			if !more {
				return nil
			}
		}
	}
}

func customMapKeyDecoder(t reflect.Type, b *decoderBuilder) func(d *decodeState, keyVal reflect.Value) error {
	keyType := t.Key()
	fns := b.cache.unmarshalers.applicable(keyType)
	if len(fns) == 0 {
		return nil
	}
	fallback := &typeDecoder{zero: zeroFuncFor(keyType), fn: func(d *decodeState, p unsafe.Pointer) error {
		key, err := d.readStringView()
		if err != nil {
			return err
		}
		return setMapKey(reflect.NewAt(keyType, p).Elem(), keyType, t, key, d)
	}}
	decode := customDecodeFunc(keyType, fns, fallback)
	return func(d *decodeState, keyVal reflect.Value) error {
		if d.buf[d.pos] != '"' {
			return d.syntaxError("expected string for object name")
		}
		keyVal.SetZero()
		if err := decode(d, keyVal.Addr().UnsafePointer()); err != nil {
			return err
		}
		return d.colon()
	}
}

func setMapKey(keyVal reflect.Value, keyType, mapType reflect.Type, key []byte, d *decodeState) error {
	switch keyType.Kind() {
	case reflect.String:
		keyVal.SetString(d.ownString(key))
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := parseIntValue(string(key), keyType, false)
		if err != nil {
			return err
		}
		keyVal.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		n, err := parseUintValue(string(key), keyType, false)
		if err != nil {
			return err
		}
		keyVal.SetUint(n)
	default:
		keyVal.SetZero()
		if u, ok := keyVal.Addr().Interface().(encoding.TextUnmarshaler); ok {
			return u.UnmarshalText([]byte(string(key)))
		}
	}
	return nil
}

func decodeNativeMap[V any](t reflect.Type, elem *typeDecoder) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		if d.buf[d.pos] != '{' {
			return newSemanticError(t, "expected object")
		}
		if err := d.enter(); err != nil {
			return err
		}
		m := *(*map[string]V)(p)
		if m == nil {
			m = make(map[string]V)
			*(*map[string]V)(p) = m
		}
		if err := d.nextValueStart(); err != nil {
			return err
		}
		if d.buf[d.pos] == '}' {
			d.pos++
			d.depth--
			return nil
		}
		value := new(V)
		var zero V
		for {
			key, err := d.objectKey()
			if err != nil {
				return err
			}
			name := d.ownString(key)
			*value = zero
			if err := d.value(elem, unsafe.Pointer(value)); err != nil {
				return err
			}
			m[name] = *value
			more, err := d.afterElement('}')
			if err != nil {
				return err
			}
			if !more {
				return nil
			}
		}
	}
}

func decodeMapStringAny(d *decodeState, p unsafe.Pointer) error {
	if d.buf[d.pos] != '{' {
		return newSemanticError(mapStringAnyType, "expected object")
	}
	m, err := d.anyObject()
	if err != nil {
		return err
	}
	*(*map[string]any)(p) = m
	return nil
}

func decodeEmptyInterface(d *decodeState, p unsafe.Pointer) error {
	v, err := d.anyValue()
	if err != nil {
		return err
	}
	*(*any)(p) = v
	return nil
}

func decodeInterfaceWith(t reflect.Type, anyValue func(d *decodeState) (any, error)) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		v, err := anyValue(d)
		if err != nil {
			return err
		}
		target := reflect.NewAt(t, p).Elem()
		if v == nil {
			target.SetZero()
			return nil
		}
		rv := reflect.ValueOf(v)
		if !rv.Type().AssignableTo(t) {
			return newSemanticError(t, "cannot assign "+rv.Type().String())
		}
		target.Set(rv)
		return nil
	}
}

func (b *decoderBuilder) dynamicValue() func(d *decodeState) (any, error) {
	objects := b.build(mapStringAnyType)
	arrays := b.build(sliceAnyType)
	strs := b.build(stringType)
	bools := b.build(boolType)
	numbers := b.build(float64Type)
	return func(d *decodeState) (any, error) {
		switch c := d.buf[d.pos]; {
		case c == '{':
			return decodeDynamic[map[string]any](d, objects)
		case c == '[':
			return decodeDynamic[[]any](d, arrays)
		case c == '"':
			return decodeDynamic[string](d, strs)
		case c == 't' || c == 'f':
			return decodeDynamic[bool](d, bools)
		case c == 'n':
			return nil, d.literal("null")
		case isNumberStart(c):
			return decodeDynamic[float64](d, numbers)
		}
		return nil, d.syntaxError("unexpected character")
	}
}

func decodeDynamic[T any](d *decodeState, td *typeDecoder) (any, error) {
	var v T
	if err := d.value(td, unsafe.Pointer(&v)); err != nil {
		return nil, err
	}
	return v, nil
}

func (d *decodeState) anyValue() (any, error) {
	switch c := d.buf[d.pos]; c {
	case '{':
		m, err := d.anyObject()
		if err != nil {
			return nil, err
		}
		return m, nil
	case '[':
		a, err := d.anyArray()
		if err != nil {
			return nil, err
		}
		return boxInSlab(&d.sliceSlab, anySliceEfaceType, a, anySliceSlabLen), nil
	case '"':
		s, err := d.readString()
		if err != nil {
			return nil, err
		}
		return boxInSlab(&d.stringSlab, stringEfaceType, s, stringSlabLen), nil
	case 't':
		return true, d.literal("true")
	case 'f':
		return false, d.literal("false")
	case 'n':
		return nil, d.literal("null")
	default:
		if !isNumberStart(c) {
			return nil, d.syntaxError("unexpected character")
		}
		s, err := d.numberText()
		if err != nil {
			return nil, err
		}
		if f, ok := fnum.ParseFloat64(s); ok {
			return boxInSlab(&d.floatSlab, float64EfaceType, f, floatSlabLen), nil
		}
		return parseAnyNumber(s)
	}
}

const (
	floatSlabLen    = 256
	stringSlabLen   = 128
	anySliceSlabLen = 32
)

func efaceTypeOf(v any) unsafe.Pointer {
	return efaceOf(&v).rtype
}

var (
	float64EfaceType  = efaceTypeOf(float64(0))
	stringEfaceType   = efaceTypeOf("")
	anySliceEfaceType = efaceTypeOf([]any(nil))
)

func boxInSlab[T any](slab *[]T, rtype unsafe.Pointer, v T, slabLen int) any {
	if len(*slab) == 0 {
		*slab = make([]T, slabLen)
	}
	slot := &(*slab)[0]
	*slab = (*slab)[1:]
	*slot = v
	var out any
	*(*eface)(unsafe.Pointer(&out)) = eface{rtype: rtype, data: unsafe.Pointer(slot)}
	return out
}

func (d *decodeState) anyObject() (map[string]any, error) {
	if err := d.enter(); err != nil {
		return nil, err
	}
	m := make(map[string]any)
	if err := d.nextValueStart(); err != nil {
		return nil, err
	}
	if d.buf[d.pos] == '}' {
		d.pos++
		d.depth--
		return m, nil
	}
	for {
		keyBytes, err := d.objectKey()
		if err != nil {
			return nil, err
		}
		key := d.ownString(keyBytes)
		v, err := d.anyValue()
		if err != nil {
			return nil, err
		}
		m[key] = v
		more, err := d.afterElement('}')
		if err != nil {
			return nil, err
		}
		if !more {
			return m, nil
		}
	}
}

func (d *decodeState) anyArray() ([]any, error) {
	if err := d.enter(); err != nil {
		return nil, err
	}
	if err := d.nextValueStart(); err != nil {
		return nil, err
	}
	if d.buf[d.pos] == ']' {
		d.pos++
		d.depth--
		return []any{}, nil
	}
	base := len(d.anyStack)
	for {
		v, err := d.anyValue()
		if err != nil {
			return nil, err
		}
		d.anyStack = append(d.anyStack, v)
		more, err := d.afterElement(']')
		if err != nil {
			return nil, err
		}
		if !more {
			break
		}
	}
	out := make([]any, len(d.anyStack)-base)
	copy(out, d.anyStack[base:])
	clear(d.anyStack[base:])
	d.anyStack = d.anyStack[:base]
	return out, nil
}

func decodeSkip(d *decodeState, _ unsafe.Pointer) error {
	return d.skipValue()
}

func decodeUnmarshalerFrom(t reflect.Type) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		dec := jsontext.NewDecoderFromBytes(d.buf[d.pos:])
		err := reflect.NewAt(t, p).Interface().(UnmarshalerFrom).UnmarshalJSONFrom(dec)
		d.pos += int(dec.InputOffset())
		jsontext.PutDecoder(dec)
		return err
	}
}

func decodeUnmarshaler(t reflect.Type) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		start := d.pos
		if err := d.skipValue(); err != nil {
			return err
		}
		return reflect.NewAt(t, p).Interface().(Unmarshaler).UnmarshalJSON(d.buf[start:d.pos])
	}
}

func decodeTextUnmarshaler(t reflect.Type, byKind decodeFunc) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		if d.buf[d.pos] != '"' {
			return byKind(d, p)
		}
		s, err := d.readStringView()
		if err != nil {
			return err
		}
		return reflect.NewAt(t, p).Interface().(encoding.TextUnmarshaler).UnmarshalText([]byte(string(s)))
	}
}
