package json

import (
	"reflect"
	"strconv"
	"strings"
	"unsafe"

	"github.com/guno1928/alos-json/internal/scan"
	"github.com/guno1928/alos-json/jsontext"
)

const (
	stdNumberPkgPath  = "encoding/json"
	stdNumberTypeName = "Number"
)

func isStdNumberType(t reflect.Type) bool {
	return t.Kind() == reflect.String && t.Name() == stdNumberTypeName && t.PkgPath() == stdNumberPkgPath
}

func isNumberText(s string) bool {
	end, ok := scan.NumberEnd(unsafeBytesFromStr(s), 0)
	return ok && end == len(s)
}

func invalidNumberError(t reflect.Type, s string) error {
	return newSemanticError(t, "invalid number literal "+strconv.Quote(s))
}

func encodeStdNumber(t reflect.Type) encodeFunc {
	return func(_ *encodeState, b []byte, p unsafe.Pointer) ([]byte, error) {
		s := *(*string)(p)
		if s == "" {
			return append(b, '0'), nil
		}
		if !isNumberText(s) {
			return b, invalidNumberError(t, s)
		}
		return append(b, s...), nil
	}
}

func decodeStdNumber(t reflect.Type) decodeFunc {
	return func(d *decodeState, p unsafe.Pointer) error {
		switch c := d.buf[d.pos]; {
		case isNumberStart(c):
			start := d.pos
			if _, err := d.numberText(); err != nil {
				return err
			}
			*(*string)(p) = d.ownString(d.buf[start:d.pos])
			return nil
		case c == '"':
			s, err := d.readString()
			if err != nil {
				return err
			}
			if !isNumberText(s) {
				return invalidNumberError(t, s)
			}
			*(*string)(p) = s
			return nil
		default:
			return kindError(t, "number", c)
		}
	}
}

func decStdNumber(dec *jsontext.Decoder, v reflect.Value, _ *Options) error {
	tok, err := dec.ReadToken()
	if err != nil {
		return err
	}
	switch tok.Kind() {
	case jsontext.Number:
		v.SetString(strings.Clone(tokenText(tok)))
	case jsontext.String:
		s := tok.Str()
		if !isNumberText(s) {
			return invalidNumberError(v.Type(), s)
		}
		v.SetString(s)
	default:
		return newSemanticError(v.Type(), "expected number, got "+tok.Kind().String())
	}
	return nil
}
