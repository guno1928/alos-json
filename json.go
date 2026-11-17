package json

import (
	"io"
	"reflect"

	"github.com/guno1928/alos-json/internal/scan"
	"github.com/guno1928/alos-json/jsontext"
)

// MarshalWrite marshals v as JSON and writes to w.
func MarshalWrite(w io.Writer, v any, opts ...Option) error {
	b, err := Marshal(v, opts...)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}

// UnmarshalRead reads JSON from r and unmarshals into v.
func UnmarshalRead(r io.Reader, v any, opts ...Option) error {
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return &SemanticError{Msg: "unmarshal requires non-nil pointer"}
	}
	o := buildOptions(opts)
	if o.unmarshalers != nil {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		return unmarshalBytesFast(data, v, opts)
	}
	dec := jsontext.NewDecoder(r)
	err := unmarshalTopLevel(dec, rv.Elem(), &o)
	jsontext.PutDecoder(dec)
	return err
}

// Valid reports whether data is structurally valid JSON. String contents are not inspected.
func Valid(data []byte) bool {
	return scan.ValidLax(data)
}

// Compact appends to dst the JSON-encoded src with insignificant space removed.
func Compact(dst *[]byte, src []byte) error {
	v := jsontext.Value(src)
	if err := v.Compact(); err != nil {
		return err
	}
	*dst = append(*dst, v...)
	return nil
}

// Indent appends to dst an indented form of the JSON-encoded src.
func Indent(dst *[]byte, src []byte, prefix, indent string) error {
	v := jsontext.Value(src)
	if err := v.Indent(prefix, indent); err != nil {
		return err
	}
	*dst = append(*dst, v...)
	return nil
}
