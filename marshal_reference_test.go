package json

import (
	"encoding"
	"encoding/base64"
	"reflect"
	"sort"
	"sync"
	"unsafe"

	"github.com/guno1928/alos-json/jsontext"
)

type directEnc func(dst *[]byte, v reflect.Value, o *Options) error

var directEncCache sync.Map

func cachedDirectEnc(t reflect.Type) directEnc {
	if v, ok := directEncCache.Load(t); ok {
		return v.(directEnc)
	}
	var wg sync.WaitGroup
	wg.Add(1)
	var fn directEnc
	fi, loaded := directEncCache.LoadOrStore(t, directEnc(func(dst *[]byte, v reflect.Value, o *Options) error {
		wg.Wait()
		return fn(dst, v, o)
	}))
	if loaded {
		return fi.(directEnc)
	}
	fn = buildDirectEnc(t)
	directEncCache.Store(t, fn)
	wg.Done()
	return fn
}

func buildDirectEnc(t reflect.Type) directEnc {
	fn := buildMethodOrKindEnc(t)
	if (t.Kind() == reflect.Pointer || t.Kind() == reflect.Interface) && hasCustomMarshaler(t) {
		return func(dst *[]byte, v reflect.Value, o *Options) error {
			if v.IsNil() {
				*dst = append(*dst, "null"...)
				return nil
			}
			return fn(dst, v, o)
		}
	}
	return fn
}

func buildMethodOrKindEnc(t reflect.Type) directEnc {
	if t.Implements(marshalerToType) {
		return dMarshalerTo
	}
	if reflect.PointerTo(t).Implements(marshalerToType) {
		return dAddrMarshalerTo
	}
	if t.Implements(marshalerType) {
		return dMarshaler
	}
	if reflect.PointerTo(t).Implements(marshalerType) {
		return dAddrMarshaler
	}
	if t.Implements(textMarshalerType) {
		return dTextMarshaler
	}
	if reflect.PointerTo(t).Implements(textMarshalerType) {
		return dAddrTextMarshaler
	}

	switch t.Kind() {
	case reflect.Bool:
		return dBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return dInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return dUint
	case reflect.Float32:
		return dFloat32
	case reflect.Float64:
		return dFloat64
	case reflect.String:
		return dString
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return dBytes
		}
		return dNewSlice(t)
	case reflect.Array:
		return dNewArray(t)
	case reflect.Map:
		return dNewMap(t)
	case reflect.Struct:
		return dNewStruct(t)
	case reflect.Pointer:
		return dNewPtr(t)
	case reflect.Interface:
		return dInterface
	default:
		return dUnsupported
	}
}

func dBool(dst *[]byte, v reflect.Value, _ *Options) error {
	if v.Bool() {
		*dst = append(*dst, "true"...)
	} else {
		*dst = append(*dst, "false"...)
	}
	return nil
}

func dInt(dst *[]byte, v reflect.Value, _ *Options) error {
	*dst = jsontext.FastAppendInt(*dst, v.Int())
	return nil
}

func dUint(dst *[]byte, v reflect.Value, _ *Options) error {
	*dst = jsontext.FastAppendUint(*dst, v.Uint())
	return nil
}

func dFloat32(dst *[]byte, v reflect.Value, _ *Options) error {
	return appendFloat(dst, v.Float(), 32)
}

func dFloat64(dst *[]byte, v reflect.Value, _ *Options) error {
	return appendFloat(dst, v.Float(), 64)
}

func dString(dst *[]byte, v reflect.Value, o *Options) error {
	*dst = o.appendStr(*dst, v.String())
	return nil
}

func dBytes(dst *[]byte, v reflect.Value, _ *Options) error {
	b := v.Bytes()
	if b == nil {
		*dst = append(*dst, "null"...)
		return nil
	}
	encodedLen := base64.StdEncoding.EncodedLen(len(b))
	*dst = append(*dst, '"')
	start := len(*dst)
	*dst = growSlice(*dst, encodedLen)
	base64.StdEncoding.Encode((*dst)[start:start+encodedLen], b)
	*dst = (*dst)[:start+encodedLen]
	*dst = append(*dst, '"')
	return nil
}

func dNewSlice(t reflect.Type) directEnc {
	elemEnc := cachedDirectEnc(t.Elem())
	isElemStruct := t.Elem().Kind() == reflect.Struct
	return func(dst *[]byte, v reflect.Value, o *Options) error {
		if v.IsNil() {
			if o.has(flagFormatNilSliceAsNull) {
				*dst = append(*dst, "null"...)
			} else {
				*dst = append(*dst, '[', ']')
			}
			return nil
		}
		*dst = append(*dst, '[')
		n := v.Len()
		if isElemStruct && n > 0 {
			if err := elemEnc(dst, v.Index(0), o); err != nil {
				return err
			}
			for i := 1; i < n; i++ {
				*dst = append(*dst, ',')
				if err := elemEnc(dst, v.Index(i), o); err != nil {
					return err
				}
			}
		} else {
			for i := 0; i < n; i++ {
				if i > 0 {
					*dst = append(*dst, ',')
				}
				if err := elemEnc(dst, v.Index(i), o); err != nil {
					return err
				}
			}
		}
		*dst = append(*dst, ']')
		return nil
	}
}

func dNewArray(t reflect.Type) directEnc {
	elemEnc := cachedDirectEnc(t.Elem())
	return func(dst *[]byte, v reflect.Value, o *Options) error {
		*dst = append(*dst, '[')
		n := v.Len()
		for i := 0; i < n; i++ {
			if i > 0 {
				*dst = append(*dst, ',')
			}
			if err := elemEnc(dst, v.Index(i), o); err != nil {
				return err
			}
		}
		*dst = append(*dst, ']')
		return nil
	}
}

func dNewMap(t reflect.Type) directEnc {
	valEnc := cachedDirectEnc(t.Elem())
	valIsIface := t.Elem().Kind() == reflect.Interface
	keyIsString := t.Key().Kind() == reflect.String
	return func(dst *[]byte, v reflect.Value, o *Options) error {
		if v.IsNil() {
			if o.has(flagFormatNilMapAsNull) {
				*dst = append(*dst, "null"...)
			} else {
				*dst = append(*dst, '{', '}')
			}
			return nil
		}
		*dst = append(*dst, '{')
		escHTML := o.has(flagEscapeHTML)
		if o.has(flagDeterministic) {
			keys := v.MapKeys()
			sort.Slice(keys, func(i, j int) bool {
				return mapKeyString(keys[i]) < mapKeyString(keys[j])
			})
			for i, key := range keys {
				if i > 0 {
					*dst = append(*dst, ',')
				}
				if keyIsString {
					if escHTML {
						*dst = jsontext.AppendQuoteStringHTML(*dst, key.String())
					} else {
						*dst = appendJStr(*dst, key.String())
					}
				} else {
					*dst = appendMapKeyStr(*dst, key)
				}
				*dst = append(*dst, ':')
				val := v.MapIndex(key)
				if valIsIface && !val.IsNil() {
					if err := dInterface(dst, val, o); err != nil {
						return err
					}
				} else {
					if err := valEnc(dst, val, o); err != nil {
						return err
					}
				}
			}
		} else {
			first := true
			iter := v.MapRange()
			for iter.Next() {
				if first {
					first = false
				} else {
					*dst = append(*dst, ',')
				}
				if keyIsString {
					if escHTML {
						*dst = jsontext.AppendQuoteStringHTML(*dst, iter.Key().String())
					} else {
						*dst = appendJStr(*dst, iter.Key().String())
					}
				} else {
					*dst = appendMapKeyStr(*dst, iter.Key())
				}
				*dst = append(*dst, ':')
				val := iter.Value()
				if valIsIface && !val.IsNil() {
					if err := dInterface(dst, val, o); err != nil {
						return err
					}
				} else {
					if err := valEnc(dst, val, o); err != nil {
						return err
					}
				}
			}
		}
		*dst = append(*dst, '}')
		return nil
	}
}

type fieldKind uint8

const (
	fkOther fieldKind = iota
	fkBool
	fkInt
	fkUint
	fkFloat32
	fkFloat64
	fkString
	fkSliceString
	fkStruct
)

func kindForType(t reflect.Type) fieldKind {
	if hasCustomMarshaler(t) {
		return fkOther
	}
	switch t.Kind() {
	case reflect.Bool:
		return fkBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return fkInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return fkUint
	case reflect.Float32:
		return fkFloat32
	case reflect.Float64:
		return fkFloat64
	case reflect.String:
		return fkString
	case reflect.Slice:
		if t.Elem().Kind() == reflect.String && !hasCustomMarshaler(t.Elem()) {
			return fkSliceString
		}
		return fkOther
	case reflect.Struct:
		return fkStruct
	default:
		return fkOther
	}
}

// unsafeStructEnc is a compiled encoder for a struct type using unsafe.Pointer.
// Each field has its own encodeFunc that writes key+value from the base pointer.
type unsafeStructEnc struct {
	fields []unsafeFieldEnc
}

type unsafeFieldEnc struct {
	key    string
	offset uintptr
	enc    func(dst *[]byte, fp unsafe.Pointer, o *Options) error
}

var unsafeStructCache sync.Map

func cachedUnsafeStructEnc(t reflect.Type) *unsafeStructEnc {
	if v, ok := unsafeStructCache.Load(t); ok {
		return v.(*unsafeStructEnc)
	}
	enc := buildUnsafeStructEnc(t)
	v, _ := unsafeStructCache.LoadOrStore(t, enc)
	return v.(*unsafeStructEnc)
}

func buildUnsafeStructEnc(t reflect.Type) *unsafeStructEnc {
	fs := cachedFields(t)
	enc := &unsafeStructEnc{
		fields: make([]unsafeFieldEnc, 0, len(fs.list)),
	}
	for i := range fs.list {
		f := &fs.list[i]
		if !f.direct || f.omitZero || f.omitEmpty || f.asString {
			// Can't use unsafe path for these
			return nil
		}
		uf := unsafeFieldEnc{
			key:    appendFieldKey(f.name),
			offset: f.offset,
		}
		uf.enc = buildUnsafeFieldWriter(f.typ)
		if uf.enc == nil {
			return nil // can't handle this field type
		}
		enc.fields = append(enc.fields, uf)
	}
	return enc
}

func buildUnsafeFieldWriter(t reflect.Type) func(dst *[]byte, fp unsafe.Pointer, o *Options) error {
	if hasCustomMarshaler(t) {
		return nil
	}
	switch t.Kind() {
	case reflect.Bool:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			if *(*bool)(fp) {
				*dst = append(*dst, "true"...)
			} else {
				*dst = append(*dst, "false"...)
			}
			return nil
		}
	case reflect.Int:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			*dst = jsontext.FastAppendInt(*dst, int64(*(*int)(fp)))
			return nil
		}
	case reflect.Int8:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			*dst = jsontext.FastAppendInt(*dst, int64(*(*int8)(fp)))
			return nil
		}
	case reflect.Int16:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			*dst = jsontext.FastAppendInt(*dst, int64(*(*int16)(fp)))
			return nil
		}
	case reflect.Int32:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			*dst = jsontext.FastAppendInt(*dst, int64(*(*int32)(fp)))
			return nil
		}
	case reflect.Int64:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			*dst = jsontext.FastAppendInt(*dst, *(*int64)(fp))
			return nil
		}
	case reflect.Uint, reflect.Uintptr:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			*dst = jsontext.FastAppendUint(*dst, uint64(*(*uint)(fp)))
			return nil
		}
	case reflect.Uint8:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			*dst = jsontext.FastAppendUint(*dst, uint64(*(*uint8)(fp)))
			return nil
		}
	case reflect.Uint16:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			*dst = jsontext.FastAppendUint(*dst, uint64(*(*uint16)(fp)))
			return nil
		}
	case reflect.Uint32:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			*dst = jsontext.FastAppendUint(*dst, uint64(*(*uint32)(fp)))
			return nil
		}
	case reflect.Uint64:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			*dst = jsontext.FastAppendUint(*dst, *(*uint64)(fp))
			return nil
		}
	case reflect.Float32:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			if err := appendFloat(dst, float64(*(*float32)(fp)), 32); err != nil {
				return err
			}
			return nil
		}
	case reflect.Float64:
		return func(dst *[]byte, fp unsafe.Pointer, _ *Options) error {
			if err := appendFloat(dst, *(*float64)(fp), 64); err != nil {
				return err
			}
			return nil
		}
	case reflect.String:
		return func(dst *[]byte, fp unsafe.Pointer, o *Options) error {
			s := *(*string)(fp)
			if o.has(flagEscapeHTML) {
				*dst = jsontext.AppendQuoteStringHTML(*dst, s)
			} else {
				*dst = appendJStr(*dst, s)
			}
			return nil
		}
	case reflect.Slice:
		if t.Elem().Kind() == reflect.String && !hasCustomMarshaler(t.Elem()) {
			return unsafeStringSliceWriter
		}
		return nil
	case reflect.Struct:
		// Recursively build an unsafe struct encoder
		nested := cachedUnsafeStructEnc(t)
		if nested == nil {
			return nil
		}
		return func(dst *[]byte, fp unsafe.Pointer, o *Options) error {
			return encodeStructUnsafe(dst, fp, nested, o)
		}
	default:
		return nil
	}
}

// stringSlice matches the memory layout of []string
type stringSlice struct {
	data unsafe.Pointer
	len  int
	cap  int
}

func unsafeStringSliceWriter(dst *[]byte, fp unsafe.Pointer, o *Options) error {
	ss := *(*stringSlice)(fp)
	if ss.data == nil {
		if o.has(flagFormatNilSliceAsNull) {
			*dst = append(*dst, "null"...)
		} else {
			*dst = append(*dst, '[', ']')
		}
		return nil
	}
	*dst = append(*dst, '[')
	escHTML := o.has(flagEscapeHTML)
	// Each string is 16 bytes on 64-bit (pointer + length)
	const strSize = unsafe.Sizeof("")
	for i := 0; i < ss.len; i++ {
		if i > 0 {
			*dst = append(*dst, ',')
		}
		sp := unsafe.Add(ss.data, uintptr(i)*strSize)
		s := *(*string)(sp)
		if escHTML {
			*dst = jsontext.AppendQuoteStringHTML(*dst, s)
		} else {
			*dst = appendJStr(*dst, s)
		}
	}
	*dst = append(*dst, ']')
	return nil
}

func encodeStructUnsafe(dst *[]byte, base unsafe.Pointer, enc *unsafeStructEnc, o *Options) error {
	*dst = append(*dst, '{')
	for i := range enc.fields {
		f := &enc.fields[i]
		if i > 0 {
			*dst = append(*dst, ',')
		}
		*dst = append(*dst, f.key...)
		fp := unsafe.Add(base, f.offset)
		if err := f.enc(dst, fp, o); err != nil {
			return err
		}
	}
	*dst = append(*dst, '}')
	return nil
}

type dFieldEnc struct {
	typ       reflect.Type
	key       string
	index     []int
	singleIdx int
	hasSingle bool
	hasOmit   bool
	omitZero  bool
	omitEmpty bool
	asString  bool
	kind      fieldKind
	enc       directEnc
	offset    uintptr
	direct    bool
	intSize   int
	uintSize  int
	unsafeEnc func(dst *[]byte, fp unsafe.Pointer, o *Options) error // for fkStruct/fkSliceString
}

func dNewStruct(t reflect.Type) directEnc {
	fs := cachedFields(t)
	directIface := isDirectIface(t)

	// Try fully-unsafe path first: if ALL fields can be encoded via pointer,
	// we skip reflect.Value entirely.
	useFullUnsafe := true
	fes := make([]dFieldEnc, len(fs.list))
	for i := range fs.list {
		f := &fs.list[i]
		fe := dFieldEnc{
			typ:       f.typ,
			key:       appendFieldKey(f.name),
			index:     f.index,
			hasSingle: len(f.index) == 1,
			hasOmit:   f.omitZero || f.omitEmpty,
			omitZero:  f.omitZero,
			omitEmpty: f.omitEmpty,
			asString:  f.asString,
			kind:      kindForType(f.typ),
			enc:       cachedDirectEnc(f.typ),
			offset:    f.offset,
			direct:    f.direct,
		}
		if len(f.index) == 1 {
			fe.singleIdx = f.index[0]
		}
		// Build unsafe encoder for struct/slice fields
		if fe.direct && !fe.hasOmit && !fe.asString {
			switch fe.kind {
			case fkStruct, fkSliceString:
				fe.unsafeEnc = buildUnsafeFieldWriter(f.typ)
			}
		}
		// If any field can't be done via unsafe, mark as not fully unsafe
		if !fe.direct || fe.hasOmit || fe.asString {
			useFullUnsafe = false
		} else if fe.kind == fkOther {
			useFullUnsafe = false
		} else if (fe.kind == fkStruct || fe.kind == fkSliceString) && fe.unsafeEnc == nil {
			useFullUnsafe = false
		}
		switch f.typ.Kind() {
		case reflect.Int:
			fe.intSize = 0
		case reflect.Int8:
			fe.intSize = 8
		case reflect.Int16:
			fe.intSize = 16
		case reflect.Int32:
			fe.intSize = 32
		case reflect.Int64:
			fe.intSize = 64
		case reflect.Uint:
			fe.uintSize = 0
		case reflect.Uint8:
			fe.uintSize = 8
		case reflect.Uint16:
			fe.uintSize = 16
		case reflect.Uint32:
			fe.uintSize = 32
		case reflect.Uint64:
			fe.uintSize = 64
		case reflect.Uintptr:
			fe.uintSize = 0
		}
		fes[i] = fe
	}

	// If all fields can be encoded via unsafe, skip reflect entirely
	if useFullUnsafe && len(fes) > 0 {
		return func(dst *[]byte, v reflect.Value, o *Options) error {
			ptr := rvDataPtr(v, directIface)
			if ptr == nil {
				// Fallback to reflect
				return encStructReflect(dst, v, fes, o)
			}
			escHTML := o.has(flagEscapeHTML)
			*dst = append(*dst, '{')
			for i := range fes {
				fe := &fes[i]
				if i > 0 {
					*dst = append(*dst, ',')
				}
				*dst = append(*dst, fe.key...)
				fp := unsafe.Add(ptr, fe.offset)
				switch fe.kind {
				case fkBool:
					if *(*bool)(fp) {
						*dst = append(*dst, "true"...)
					} else {
						*dst = append(*dst, "false"...)
					}
				case fkInt:
					switch fe.intSize {
					case 0:
						*dst = jsontext.FastAppendInt(*dst, int64(*(*int)(fp)))
					case 8:
						*dst = jsontext.FastAppendInt(*dst, int64(*(*int8)(fp)))
					case 16:
						*dst = jsontext.FastAppendInt(*dst, int64(*(*int16)(fp)))
					case 32:
						*dst = jsontext.FastAppendInt(*dst, int64(*(*int32)(fp)))
					case 64:
						*dst = jsontext.FastAppendInt(*dst, *(*int64)(fp))
					}
				case fkUint:
					switch fe.uintSize {
					case 0:
						*dst = jsontext.FastAppendUint(*dst, uint64(*(*uint)(fp)))
					case 8:
						*dst = jsontext.FastAppendUint(*dst, uint64(*(*uint8)(fp)))
					case 16:
						*dst = jsontext.FastAppendUint(*dst, uint64(*(*uint16)(fp)))
					case 32:
						*dst = jsontext.FastAppendUint(*dst, uint64(*(*uint32)(fp)))
					case 64:
						*dst = jsontext.FastAppendUint(*dst, *(*uint64)(fp))
					}
				case fkFloat32:
					if err := appendFloat(dst, float64(*(*float32)(fp)), 32); err != nil {
						return err
					}
				case fkFloat64:
					if err := appendFloat(dst, *(*float64)(fp), 64); err != nil {
						return err
					}
				case fkString:
					s := *(*string)(fp)
					if escHTML {
						*dst = jsontext.AppendQuoteStringHTML(*dst, s)
					} else {
						*dst = appendJStr(*dst, s)
					}
				case fkSliceString, fkStruct:
					if err := fe.unsafeEnc(dst, fp, o); err != nil {
						return err
					}
				}
			}
			*dst = append(*dst, '}')
			return nil
		}
	}

	return func(dst *[]byte, v reflect.Value, o *Options) error {
		*dst = append(*dst, '{')
		first := true
		escHTML := o.has(flagEscapeHTML)
		ptr := rvDataPtr(v, directIface)
		for i := range fes {
			fe := &fes[i]

			if ptr != nil && fe.direct && !fe.hasOmit && !fe.asString {
				fp := unsafe.Add(ptr, fe.offset)
				if first {
					first = false
				} else {
					*dst = append(*dst, ',')
				}
				*dst = append(*dst, fe.key...)
				switch fe.kind {
				case fkBool:
					if *(*bool)(fp) {
						*dst = append(*dst, "true"...)
					} else {
						*dst = append(*dst, "false"...)
					}
					continue
				case fkInt:
					var n int64
					switch fe.intSize {
					case 0:
						n = int64(*(*int)(fp))
					case 8:
						n = int64(*(*int8)(fp))
					case 16:
						n = int64(*(*int16)(fp))
					case 32:
						n = int64(*(*int32)(fp))
					case 64:
						n = *(*int64)(fp)
					}
					*dst = jsontext.FastAppendInt(*dst, n)
					continue
				case fkUint:
					var n uint64
					switch fe.uintSize {
					case 0:
						n = uint64(*(*uint)(fp))
					case 8:
						n = uint64(*(*uint8)(fp))
					case 16:
						n = uint64(*(*uint16)(fp))
					case 32:
						n = uint64(*(*uint32)(fp))
					case 64:
						n = *(*uint64)(fp)
					}
					*dst = jsontext.FastAppendUint(*dst, n)
					continue
				case fkFloat32:
					if err := appendFloat(dst, float64(*(*float32)(fp)), 32); err != nil {
						return err
					}
					continue
				case fkFloat64:
					if err := appendFloat(dst, *(*float64)(fp), 64); err != nil {
						return err
					}
					continue
				case fkString:
					s := *(*string)(fp)
					if escHTML {
						*dst = jsontext.AppendQuoteStringHTML(*dst, s)
					} else {
						*dst = appendJStr(*dst, s)
					}
					continue
				case fkSliceString, fkStruct:
					if fe.unsafeEnc != nil {
						if err := fe.unsafeEnc(dst, fp, o); err != nil {
							return err
						}
						continue
					}
				}
				// fkOther or unsupported: fall through to reflect path below
				fv := reflect.NewAt(fe.typ, fp).Elem()
				if err := fe.enc(dst, fv, o); err != nil {
					return err
				}
				continue
			}

			var fv reflect.Value
			switch {
			case ptr != nil && fe.direct:
				fv = reflect.NewAt(fe.typ, unsafe.Add(ptr, fe.offset)).Elem()
			case fe.hasSingle:
				fv = v.Field(fe.singleIdx)
			default:
				fv = fieldByIndex(v, fe.index)
				if !fv.IsValid() {
					continue
				}
			}
			if fe.hasOmit {
				if fe.omitZero && isZero(fv) {
					continue
				}
				if fe.omitEmpty && isEmpty(fv) {
					continue
				}
			}
			if first {
				first = false
			} else {
				*dst = append(*dst, ',')
			}
			*dst = append(*dst, fe.key...)
			if fe.asString {
				if err := dMarshalAsStr(dst, fv, o); err != nil {
					return err
				}
				continue
			}
			switch fe.kind {
			case fkBool:
				if fv.Bool() {
					*dst = append(*dst, "true"...)
				} else {
					*dst = append(*dst, "false"...)
				}
			case fkInt:
				*dst = jsontext.FastAppendInt(*dst, fv.Int())
			case fkUint:
				*dst = jsontext.FastAppendUint(*dst, fv.Uint())
			case fkFloat32:
				if err := appendFloat(dst, fv.Float(), 32); err != nil {
					return err
				}
			case fkFloat64:
				if err := appendFloat(dst, fv.Float(), 64); err != nil {
					return err
				}
			case fkString:
				if escHTML {
					*dst = jsontext.AppendQuoteStringHTML(*dst, fv.String())
				} else {
					*dst = appendJStr(*dst, fv.String())
				}
			default:
				if err := fe.enc(dst, fv, o); err != nil {
					return err
				}
			}
		}
		*dst = append(*dst, '}')
		return nil
	}
}

func encStructReflect(dst *[]byte, v reflect.Value, fes []dFieldEnc, o *Options) error {
	*dst = append(*dst, '{')
	first := true
	escHTML := o.has(flagEscapeHTML)
	for i := range fes {
		fe := &fes[i]
		var fv reflect.Value
		if fe.hasSingle {
			fv = v.Field(fe.singleIdx)
		} else {
			fv = fieldByIndex(v, fe.index)
			if !fv.IsValid() {
				continue
			}
		}
		if fe.hasOmit {
			if fe.omitZero && isZero(fv) {
				continue
			}
			if fe.omitEmpty && isEmpty(fv) {
				continue
			}
		}
		if first {
			first = false
		} else {
			*dst = append(*dst, ',')
		}
		*dst = append(*dst, fe.key...)
		if fe.asString {
			if err := dMarshalAsStr(dst, fv, o); err != nil {
				return err
			}
			continue
		}
		switch fe.kind {
		case fkBool:
			if fv.Bool() {
				*dst = append(*dst, "true"...)
			} else {
				*dst = append(*dst, "false"...)
			}
		case fkInt:
			*dst = jsontext.FastAppendInt(*dst, fv.Int())
		case fkUint:
			*dst = jsontext.FastAppendUint(*dst, fv.Uint())
		case fkFloat32:
			if err := appendFloat(dst, fv.Float(), 32); err != nil {
				return err
			}
		case fkFloat64:
			if err := appendFloat(dst, fv.Float(), 64); err != nil {
				return err
			}
		case fkString:
			if escHTML {
				*dst = jsontext.AppendQuoteStringHTML(*dst, fv.String())
			} else {
				*dst = appendJStr(*dst, fv.String())
			}
		default:
			if err := fe.enc(dst, fv, o); err != nil {
				return err
			}
		}
	}
	*dst = append(*dst, '}')
	return nil
}

func dNewPtr(t reflect.Type) directEnc {
	elemEnc := cachedDirectEnc(t.Elem())
	return func(dst *[]byte, v reflect.Value, o *Options) error {
		if v.IsNil() {
			*dst = append(*dst, "null"...)
			return nil
		}
		return elemEnc(dst, v.Elem(), o)
	}
}

func dInterface(dst *[]byte, v reflect.Value, o *Options) error {
	if v.IsNil() {
		*dst = append(*dst, "null"...)
		return nil
	}
	elem := v.Elem()
	if !isPredeclaredScalar(elem.Type()) {
		return cachedDirectEnc(elem.Type())(dst, elem, o)
	}
	switch elem.Kind() {
	case reflect.Bool:
		if elem.Bool() {
			*dst = append(*dst, "true"...)
		} else {
			*dst = append(*dst, "false"...)
		}
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		*dst = jsontext.FastAppendInt(*dst, elem.Int())
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		*dst = jsontext.FastAppendUint(*dst, elem.Uint())
		return nil
	case reflect.Float32:
		if err := appendFloat(dst, elem.Float(), 32); err != nil {
			return err
		}
		return nil
	case reflect.Float64:
		if err := appendFloat(dst, elem.Float(), 64); err != nil {
			return err
		}
		return nil
	case reflect.String:
		if o.has(flagEscapeHTML) {
			*dst = jsontext.AppendQuoteStringHTML(*dst, elem.String())
		} else {
			*dst = appendJStr(*dst, elem.String())
		}
		return nil
	default:
		return cachedDirectEnc(elem.Type())(dst, elem, o)
	}
}

var predeclaredScalars = map[reflect.Type]bool{
	reflect.TypeFor[bool]():    true,
	reflect.TypeFor[int]():     true,
	reflect.TypeFor[int8]():    true,
	reflect.TypeFor[int16]():   true,
	reflect.TypeFor[int32]():   true,
	reflect.TypeFor[int64]():   true,
	reflect.TypeFor[uint]():    true,
	reflect.TypeFor[uint8]():   true,
	reflect.TypeFor[uint16]():  true,
	reflect.TypeFor[uint32]():  true,
	reflect.TypeFor[uint64]():  true,
	reflect.TypeFor[uintptr](): true,
	reflect.TypeFor[float32](): true,
	reflect.TypeFor[float64](): true,
	reflect.TypeFor[string]():  true,
}

func isPredeclaredScalar(t reflect.Type) bool {
	return predeclaredScalars[t]
}

func dUnsupported(_ *[]byte, v reflect.Value, _ *Options) error {
	return newSemanticError(v.Type(), "unsupported type")
}

func dMarshalerTo(dst *[]byte, v reflect.Value, _ *Options) error {
	enc := jsontext.WrapBuffer(*dst)
	err := v.Interface().(MarshalerTo).MarshalJSONTo(enc)
	*dst = enc.Bytes()
	jsontext.PutEncoder(enc)
	return err
}

func dAddrMarshalerTo(dst *[]byte, v reflect.Value, _ *Options) error {
	enc := jsontext.WrapBuffer(*dst)
	var err error
	if v.CanAddr() {
		err = v.Addr().Interface().(MarshalerTo).MarshalJSONTo(enc)
	} else {
		c := reflect.New(v.Type())
		c.Elem().Set(v)
		err = c.Interface().(MarshalerTo).MarshalJSONTo(enc)
	}
	*dst = enc.Bytes()
	jsontext.PutEncoder(enc)
	return err
}

func dMarshaler(dst *[]byte, v reflect.Value, _ *Options) error {
	b, err := v.Interface().(Marshaler).MarshalJSON()
	if err != nil {
		return err
	}
	*dst = append(*dst, b...)
	return nil
}

func dAddrMarshaler(dst *[]byte, v reflect.Value, _ *Options) error {
	var b []byte
	var err error
	if v.CanAddr() {
		b, err = v.Addr().Interface().(Marshaler).MarshalJSON()
	} else {
		c := reflect.New(v.Type())
		c.Elem().Set(v)
		b, err = c.Interface().(Marshaler).MarshalJSON()
	}
	if err != nil {
		return err
	}
	*dst = append(*dst, b...)
	return nil
}

func dTextMarshaler(dst *[]byte, v reflect.Value, o *Options) error {
	b, err := v.Interface().(encoding.TextMarshaler).MarshalText()
	if err != nil {
		return err
	}
	*dst = o.appendStr(*dst, b2s(b))
	return nil
}

func dAddrTextMarshaler(dst *[]byte, v reflect.Value, o *Options) error {
	var b []byte
	var err error
	if v.CanAddr() {
		b, err = v.Addr().Interface().(encoding.TextMarshaler).MarshalText()
	} else {
		c := reflect.New(v.Type())
		c.Elem().Set(v)
		b, err = c.Interface().(encoding.TextMarshaler).MarshalText()
	}
	if err != nil {
		return err
	}
	*dst = o.appendStr(*dst, b2s(b))
	return nil
}

func dMarshalAsStr(dst *[]byte, v reflect.Value, o *Options) error {
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			*dst = append(*dst, "null"...)
			return nil
		}
		v = v.Elem()
	}
	switch v.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		*dst = append(*dst, '"')
		*dst = jsontext.FastAppendInt(*dst, v.Int())
		*dst = append(*dst, '"')
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		*dst = append(*dst, '"')
		*dst = jsontext.FastAppendUint(*dst, v.Uint())
		*dst = append(*dst, '"')
	case reflect.Float32:
		*dst = append(*dst, '"')
		if err := appendFloat(dst, v.Float(), 32); err != nil {
			return err
		}
		*dst = append(*dst, '"')
	case reflect.Float64:
		*dst = append(*dst, '"')
		if err := appendFloat(dst, v.Float(), 64); err != nil {
			return err
		}
		*dst = append(*dst, '"')
	case reflect.Bool:
		if v.Bool() {
			*dst = append(*dst, `"true"`...)
		} else {
			*dst = append(*dst, `"false"`...)
		}
	default:
		return cachedDirectEnc(v.Type())(dst, v, o)
	}
	return nil
}

type encState struct {
	buf  []byte
	opts Options
}

var encPool = sync.Pool{
	New: func() any {
		return &encState{buf: make([]byte, 0, 4096)}
	},
}

func marshalReference(v any, opts ...Option) ([]byte, error) {
	es := encPool.Get().(*encState)
	es.buf = es.buf[:0]
	es.opts = Options{}
	for _, fn := range opts {
		fn(&es.opts)
	}

	if v == nil {
		es.buf = append(es.buf, "null"...)
		out := make([]byte, len(es.buf))
		copy(out, es.buf)
		encPool.Put(es)
		return out, nil
	}

	rv := reflect.ValueOf(v)
	var err error
	if !rv.IsValid() {
		es.buf = append(es.buf, "null"...)
	} else {
		for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
			if rv.IsNil() {
				es.buf = append(es.buf, "null"...)
				goto done
			}
			rv = rv.Elem()
		}
		err = cachedDirectEnc(rv.Type())(&es.buf, rv, &es.opts)
	}
done:
	if err != nil {
		encPool.Put(es)
		return nil, err
	}
	out := make([]byte, len(es.buf))
	copy(out, es.buf)
	encPool.Put(es)
	return out, nil
}

func marshalAppendReference(dst []byte, v any, opts ...Option) ([]byte, error) {
	var o Options
	for _, fn := range opts {
		fn(&o)
	}
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return append(dst, "null"...), nil
	}
	for rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return append(dst, "null"...), nil
		}
		rv = rv.Elem()
	}
	if err := cachedDirectEnc(rv.Type())(&dst, rv, &o); err != nil {
		return dst, err
	}
	return dst, nil
}

// rvDataPtr extracts the data pointer from a reflect.Value.
// For addressable values, uses v.Addr().UnsafePointer().
// For non-addressable struct values, extracts the internal ptr field
// from the reflect.Value layout. This is safe because Go's interface
// boxing always heap-allocates struct values, and the reflect.Value
// stores a pointer to that allocation.
func rvDataPtr(v reflect.Value, directIface bool) unsafe.Pointer {
	if v.CanAddr() {
		return v.Addr().UnsafePointer()
	}
	if v.Kind() != reflect.Struct || directIface {
		return nil
	}
	// reflect.Value layout: { typ *rtype; ptr unsafe.Pointer; flag uintptr }
	type reflectValue struct {
		_   unsafe.Pointer // typ
		ptr unsafe.Pointer // data pointer
		_   uintptr        // flag
	}
	return (*reflectValue)(unsafe.Pointer(&v)).ptr
}

func (o *Options) appendStr(dst []byte, s string) []byte {
	if o.has(flagEscapeHTML) {
		return jsontext.AppendQuoteStringHTML(dst, s)
	}
	return appendJStr(dst, s)
}
