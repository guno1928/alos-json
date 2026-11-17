//go:build gc && (go1.26 || goexperiment.swissmap)

package json

import (
	"reflect"
	"unsafe"
)

const runtimeMapHooks = true

type runtimeMapIter struct {
	key  unsafe.Pointer
	elem unsafe.Pointer
	_    [2]unsafe.Pointer
}

//go:linkname mapiterinit runtime.mapiterinit
//go:noescape
func mapiterinit(mapType unsafe.Pointer, m unsafe.Pointer, it *runtimeMapIter)

//go:linkname mapiternext runtime.mapiternext
//go:noescape
func mapiternext(it *runtimeMapIter)

//go:linkname mapassign runtime.mapassign
//go:noescape
func mapassign(mapType, m, key unsafe.Pointer) unsafe.Pointer

//go:linkname mapassignFast32 runtime.mapassign_fast32
//go:noescape
func mapassignFast32(mapType, m unsafe.Pointer, key uint32) unsafe.Pointer

//go:linkname mapassignFast64 runtime.mapassign_fast64
//go:noescape
func mapassignFast64(mapType, m unsafe.Pointer, key uint64) unsafe.Pointer

//go:linkname mapassignFaststr runtime.mapassign_faststr
func mapassignFaststr(mapType, m unsafe.Pointer, key string) unsafe.Pointer

//go:linkname typedmemmove runtime.typedmemmove
//go:noescape
func typedmemmove(typ, dst, src unsafe.Pointer)

const maxFastMapElemBytes = 128

func linkedMapResizer(t reflect.Type) func(p unsafe.Pointer, size int) {
	mapType, elemType := typePointer(t), typePointer(t.Elem())
	return func(p unsafe.Pointer, size int) {
		resized := reflect.MakeMapWithSize(t, size)
		dst := resized.UnsafePointer()
		var it runtimeMapIter
		for mapiterinit(mapType, *(*unsafe.Pointer)(p), &it); it.key != nil; mapiternext(&it) {
			typedmemmove(elemType, mapassign(mapType, dst, it.key), it.elem)
		}
		reflect.NewAt(t, p).Elem().Set(resized)
	}
}

func linkedMapInserter(t reflect.Type) mapInserter {
	mapType, elemType := typePointer(t), typePointer(t.Elem())
	keyType := t.Key()
	if t.Elem().Size() <= maxFastMapElemBytes {
		switch {
		case keyType.Kind() == reflect.String:
			return func(m, key, elem unsafe.Pointer) {
				typedmemmove(elemType, mapassignFaststr(mapType, m, *(*string)(key)), elem)
			}
		case isIntegerKind(keyType.Kind()) && keyType.Size() == 8:
			return func(m, key, elem unsafe.Pointer) {
				typedmemmove(elemType, mapassignFast64(mapType, m, *(*uint64)(key)), elem)
			}
		case isIntegerKind(keyType.Kind()) && keyType.Size() == 4:
			return func(m, key, elem unsafe.Pointer) {
				typedmemmove(elemType, mapassignFast32(mapType, m, *(*uint32)(key)), elem)
			}
		}
	}
	return func(m, key, elem unsafe.Pointer) {
		typedmemmove(elemType, mapassign(mapType, m, key), elem)
	}
}

type mapKeyPtrEncoder func(e *encodeState, b []byte, key unsafe.Pointer) ([]byte, error)

func linkedMapEncoder(t reflect.Type, b *encoderBuilder) encodeFunc {
	if storageMayBeMutated(t.Elem()) {
		return nil
	}
	key, ok := mapKeyPtrEncoderFor(t.Key())
	if !ok {
		return nil
	}
	sorted := encodeReflectMap(t, b)
	elem := b.build(t.Elem())
	mapType := typePointer(t)
	return func(e *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		if e.opts.has(flagDeterministic) {
			return sorted(e, b, p)
		}
		m := *(*unsafe.Pointer)(p)
		if m == nil {
			return e.nilMap(b), nil
		}
		var it runtimeMapIter
		mapiterinit(mapType, m, &it)
		b = append(b, '{')
		var err error
		for ; it.key != nil; mapiternext(&it) {
			if b, err = key(e, b, it.key); err != nil {
				return b, err
			}
			if b, err = elem.fn(e, append(b, ':'), it.elem); err != nil {
				return b, err
			}
			b = append(b, ',')
		}
		if b[len(b)-1] == ',' {
			b[len(b)-1] = '}'
			return b, nil
		}
		return append(b, '}'), nil
	}
}

func mapKeyPtrEncoderFor(keyType reflect.Type) (mapKeyPtrEncoder, bool) {
	if _, err := mapKeyNamerFor(keyType); err != nil {
		return nil, false
	}
	if !usesTextMarshalerKey(keyType) {
		switch keyType.Kind() {
		case reflect.String:
			return encodeStringMapKey, true
		case reflect.Int:
			return encodeSignedMapKey[int], true
		case reflect.Int8:
			return encodeSignedMapKey[int8], true
		case reflect.Int16:
			return encodeSignedMapKey[int16], true
		case reflect.Int32:
			return encodeSignedMapKey[int32], true
		case reflect.Int64:
			return encodeSignedMapKey[int64], true
		case reflect.Uint:
			return encodeUnsignedMapKey[uint], true
		case reflect.Uint8:
			return encodeUnsignedMapKey[uint8], true
		case reflect.Uint16:
			return encodeUnsignedMapKey[uint16], true
		case reflect.Uint32:
			return encodeUnsignedMapKey[uint32], true
		case reflect.Uint64:
			return encodeUnsignedMapKey[uint64], true
		case reflect.Uintptr:
			return encodeUnsignedMapKey[uintptr], true
		}
	}
	byValue := defaultMapKeyEncoder(keyType)
	return func(e *encodeState, b []byte, key unsafe.Pointer) ([]byte, error) {
		return byValue(e, b, reflect.NewAt(keyType, key).Elem())
	}, true
}

func encodeStringMapKey(e *encodeState, b []byte, key unsafe.Pointer) ([]byte, error) {
	return e.appendString(b, *(*string)(key)), nil
}

func encodeSignedMapKey[T int | int8 | int16 | int32 | int64](e *encodeState, b []byte, key unsafe.Pointer) ([]byte, error) {
	return append(e.appendInt(append(b, '"'), int64(*(*T)(key))), '"'), nil
}

func encodeUnsignedMapKey[T uint | uint8 | uint16 | uint32 | uint64 | uintptr](e *encodeState, b []byte, key unsafe.Pointer) ([]byte, error) {
	return append(e.appendUint(append(b, '"'), uint64(*(*T)(key))), '"'), nil
}

var addressedMethodTypes = []reflect.Type{marshalerToType, marshalerType, textMarshalerType, isZeroerType}

func storageMayBeMutated(t reflect.Type) bool {
	pt := reflect.PointerTo(t)
	for _, iface := range addressedMethodTypes {
		if !t.Implements(iface) && pt.Implements(iface) {
			return true
		}
	}
	switch t.Kind() {
	case reflect.Struct:
		for i := range t.NumField() {
			if storageMayBeMutated(t.Field(i).Type) {
				return true
			}
		}
	case reflect.Array:
		return storageMayBeMutated(t.Elem())
	}
	return false
}
