package json

import (
	"reflect"

	"github.com/guno1928/alos-json/jsontext"
)

func UnmarshalReference(data []byte, v any, opts ...Option) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &SemanticError{Msg: "unmarshal requires non-nil pointer"}
	}
	o := buildOptions(opts)
	dec := jsontext.NewDecoderFromBytes(data)
	err := unmarshalTopLevel(dec, rv.Elem(), &o)
	jsontext.PutDecoder(dec)
	return err
}

func MarshalReference(v any, opts ...Option) ([]byte, error) {
	return marshalReference(v, opts...)
}

func MarshalAppendReference(dst []byte, v any, opts ...Option) ([]byte, error) {
	return marshalAppendReference(dst, v, opts...)
}
