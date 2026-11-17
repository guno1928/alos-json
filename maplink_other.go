//go:build !(gc && (go1.26 || goexperiment.swissmap))

package json

import (
	"reflect"
	"unsafe"
)

const runtimeMapHooks = false

func linkedMapEncoder(reflect.Type, *encoderBuilder) encodeFunc {
	return nil
}

func linkedMapInserter(reflect.Type) mapInserter {
	return nil
}

func linkedMapResizer(reflect.Type) func(unsafe.Pointer, int) {
	return nil
}
