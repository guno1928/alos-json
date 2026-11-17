package json

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"unsafe"

	"github.com/guno1928/alos-json/internal/scan"
	"github.com/guno1928/alos-json/jsontext"
)

var SkipFunc = errors.New("json: skip function")

var (
	errSkipMutation           = errors.New("must not read or write any tokens when skipping")
	errNonSingularValue       = errors.New("must read or write exactly one value")
	errInvalidMarshalerOutput = errors.New("MarshalJSON returned invalid JSON")
)

type marshalFn func(e *encodeState, b []byte, from reflect.Type, p unsafe.Pointer) ([]byte, error)

type unmarshalFn func(d *decodeState, from reflect.Type, p unsafe.Pointer) error

type typedFunc[F any] struct {
	typ     reflect.Type
	fn      F
	maySkip bool
}

type Marshalers struct {
	fns    []typedFunc[marshalFn]
	onces  [encoderVariants]sync.Once
	caches [encoderVariants]*encoderCache
}

type Unmarshalers struct {
	fns   []typedFunc[unmarshalFn]
	once  sync.Once
	cache *decoderCache
}

func JoinMarshalers(ms ...*Marshalers) *Marshalers {
	out := &Marshalers{}
	for _, m := range ms {
		if m != nil {
			out.fns = append(out.fns, m.fns...)
		}
	}
	return out
}

func JoinUnmarshalers(us ...*Unmarshalers) *Unmarshalers {
	out := &Unmarshalers{}
	for _, u := range us {
		if u != nil {
			out.fns = append(out.fns, u.fns...)
		}
	}
	return out
}

func MarshalFunc[T any](fn func(T) ([]byte, error)) *Marshalers {
	t := reflect.TypeFor[T]()
	assertCastableTo(t, true)
	marshal := func(_ *encodeState, b []byte, from reflect.Type, p unsafe.Pointer) ([]byte, error) {
		out, err := fn(castTo(from, p, t).(T))
		if err != nil {
			return b, funcError(t, notSkippable(err, "marshal function of type func(T) ([]byte, error)"))
		}
		v := jsontext.Value(out)
		if err := v.Compact(); err != nil {
			return b, funcError(t, err)
		}
		return append(b, v...), nil
	}
	return &Marshalers{fns: []typedFunc[marshalFn]{{typ: t, fn: marshal}}}
}

func MarshalToFunc[T any](fn func(*jsontext.Encoder, T) error) *Marshalers {
	t := reflect.TypeFor[T]()
	assertCastableTo(t, true)
	marshal := func(_ *encodeState, b []byte, from reflect.Type, p unsafe.Pointer) ([]byte, error) {
		start := len(b)
		enc := jsontext.WrapBuffer(b)
		err := fn(enc, castTo(from, p, t).(T))
		b = enc.Bytes()
		jsontext.PutEncoder(enc)
		switch {
		case err == SkipFunc && len(b) == start:
			return b, SkipFunc
		case err == SkipFunc:
			err = errSkipMutation
		case err == nil && !scan.Valid(b[start:]):
			err = errNonSingularValue
		}
		if err != nil {
			return b, funcError(t, err)
		}
		return b, nil
	}
	return &Marshalers{fns: []typedFunc[marshalFn]{{typ: t, fn: marshal, maySkip: true}}}
}

func UnmarshalFunc[T any](fn func([]byte, T) error) *Unmarshalers {
	t := reflect.TypeFor[T]()
	assertCastableTo(t, false)
	unmarshal := func(d *decodeState, from reflect.Type, p unsafe.Pointer) error {
		start := d.pos
		if err := d.skipValue(); err != nil {
			return err
		}
		if err := fn(d.buf[start:d.pos:d.pos], castTo(from, p, t).(T)); err != nil {
			return funcError(t, notSkippable(err, "unmarshal function of type func([]byte, T) error"))
		}
		return nil
	}
	return &Unmarshalers{fns: []typedFunc[unmarshalFn]{{typ: t, fn: unmarshal}}}
}

func UnmarshalFromFunc[T any](fn func(*jsontext.Decoder, T) error) *Unmarshalers {
	t := reflect.TypeFor[T]()
	assertCastableTo(t, false)
	unmarshal := func(d *decodeState, from reflect.Type, p unsafe.Pointer) error {
		dec := jsontext.NewDecoderFromBytes(d.buf[d.pos:len(d.buf):len(d.buf)])
		err := fn(dec, castTo(from, p, t).(T))
		consumed, depth := int(dec.InputOffset()), dec.StackDepth()
		jsontext.PutDecoder(dec)
		switch {
		case err == SkipFunc && consumed == 0:
			return SkipFunc
		case err == SkipFunc:
			err = errSkipMutation
		case err == nil && (consumed == 0 || depth != 0):
			err = errNonSingularValue
		}
		if err != nil {
			return funcError(t, err)
		}
		d.pos += consumed
		return nil
	}
	return &Unmarshalers{fns: []typedFunc[unmarshalFn]{{typ: t, fn: unmarshal, maySkip: true}}}
}

const (
	encoderVariantFlags = flagStringifyNumbers | flagOmitZeroStructFields
	encoderVariants     = 4
)

var encoderVariantCaches = func() (caches [encoderVariants]*encoderCache) {
	for i := range caches {
		caches[i] = newEncoderCache(nil, encoderVariantFlagsAt(i))
	}
	return caches
}()

var defaultEncoders = encoderVariantCaches[0]

func encoderVariantIndex(flags uint64) int {
	i := 0
	if flags&flagStringifyNumbers != 0 {
		i |= 1
	}
	if flags&flagOmitZeroStructFields != 0 {
		i |= 2
	}
	return i
}

func encoderVariantFlagsAt(i int) uint64 {
	var flags uint64
	if i&1 != 0 {
		flags |= flagStringifyNumbers
	}
	if i&2 != 0 {
		flags |= flagOmitZeroStructFields
	}
	return flags
}

func encodersFor(o *Options) *encoderCache {
	if o.marshalers == nil && o.flags&encoderVariantFlags == 0 {
		return defaultEncoders
	}
	return variantEncoders(o)
}

func variantEncoders(o *Options) *encoderCache {
	i := encoderVariantIndex(o.flags)
	m := o.marshalers
	if m == nil || len(m.fns) == 0 {
		return encoderVariantCaches[i]
	}
	m.onces[i].Do(func() { m.caches[i] = newEncoderCache(m, encoderVariantFlagsAt(i)) })
	return m.caches[i]
}

func decodersFor(u *Unmarshalers) *decoderCache {
	if u == nil {
		return &defaultDecoders
	}
	return u.decoders()
}

func (u *Unmarshalers) decoders() *decoderCache {
	if len(u.fns) == 0 {
		return &defaultDecoders
	}
	u.once.Do(func() { u.cache = &decoderCache{unmarshalers: u} })
	return u.cache
}

func (m *Marshalers) applicable(t reflect.Type) []typedFunc[marshalFn] {
	if m == nil {
		return nil
	}
	return applicableFuncs(m.fns, t)
}

func (u *Unmarshalers) applicable(t reflect.Type) []typedFunc[unmarshalFn] {
	if u == nil {
		return nil
	}
	return applicableFuncs(u.fns, t)
}

func applicableFuncs[F any](fns []typedFunc[F], t reflect.Type) []typedFunc[F] {
	var out []typedFunc[F]
	for _, f := range fns {
		if !castableTo(t, f.typ) {
			continue
		}
		out = append(out, f)
		if !f.maySkip {
			break
		}
	}
	return out
}

func customEncodeFunc(t reflect.Type, fns []typedFunc[marshalFn], fallback encodeFunc) encodeFunc {
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		for i := range fns {
			out, err := fns[i].fn(e, b, t, p)
			if err != SkipFunc {
				return out, err
			}
			b = out
		}
		return fallback(e, b, p)
	}
}

func customDecodeFunc(t reflect.Type, fns []typedFunc[unmarshalFn], fallback *typeDecoder) decodeFunc {
	decode, zero := fallback.fn, fallback.zero
	return func(d *decodeState, p unsafe.Pointer) error {
		for i := range fns {
			if err := fns[i].fn(d, t, p); err != SkipFunc {
				return err
			}
		}
		if d.buf[d.pos] == 'n' {
			if err := d.literal("null"); err != nil {
				return err
			}
			zero(p)
			return nil
		}
		return decode(d, p)
	}
}

func castableTo(from, to reflect.Type) bool {
	switch to.Kind() {
	case reflect.Interface:
		return reflect.PointerTo(from).Implements(to)
	case reflect.Pointer:
		return reflect.PointerTo(from) == to
	default:
		return from == to
	}
}

func castTo(from reflect.Type, p unsafe.Pointer, to reflect.Type) any {
	if k := to.Kind(); k == reflect.Interface || k == reflect.Pointer {
		return reflect.NewAt(from, p).Interface()
	}
	return reflect.NewAt(from, p).Elem().Interface()
}

func assertCastableTo(to reflect.Type, marshal bool) {
	switch to.Kind() {
	case reflect.Interface:
		return
	case reflect.Pointer:
		if to.Name() == "" {
			return
		}
	default:
		if marshal {
			return
		}
	}
	if marshal {
		panic(fmt.Sprintf("input type %v must be an interface type, an unnamed pointer type, or a non-pointer type", to))
	}
	panic(fmt.Sprintf("input type %v must be an interface type or an unnamed pointer type", to))
}

func notSkippable(err error, what string) error {
	if err == SkipFunc {
		return errors.New(what + " cannot be skipped")
	}
	return err
}

func funcError(t reflect.Type, err error) error {
	switch err.(type) {
	case *SemanticError, *jsontext.SyntacticError:
		return err
	}
	return &SemanticError{GoType: t, Err: err}
}
