package jsontext

import (
	"bytes"
	"io"

	"github.com/guno1928/alos-json/internal/scan"
)

type Value []byte

func (v Value) Kind() Kind {
	for i := 0; i < len(v); i++ {
		if v[i] > ' ' {
			return normalizeKind(v[i])
		}
	}
	return Invalid
}

func (v Value) IsValid() bool {
	return scan.Valid(v)
}

func (v Value) Clone() Value {
	out := make(Value, len(v))
	copy(out, v)
	return out
}

func (v Value) String() string {
	return unsafeStringFromBytes(v)
}

func (v *Value) Compact() error {
	src := *v
	d := NewDecoderFromBytes(src)
	defer PutDecoder(d)
	val, err := d.ReadValue()
	if err != nil {
		return err
	}
	if err := d.expectEOF(); err != nil {
		return err
	}
	*v = compactValue(make([]byte, 0, len(val)), val)
	return nil
}

func (d *Decoder) expectEOF() error {
	_, err := d.prepare()
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	return d.syntaxError("invalid data after top-level value")
}

func compactValue(dst, src []byte) []byte {
	i := 0
	for i < len(src) {
		c := src[i]
		switch {
		case c <= ' ' && (c == ' ' || c == '\t' || c == '\n' || c == '\r'):
			i++
			continue
		case c == '"':
			j := consumeString(src[i:])
			dst = append(dst, src[i:i+j]...)
			i += j
		default:
			dst = append(dst, c)
			i++
		}
	}
	return dst
}

func (v *Value) Indent(prefix, indent string) error {
	src := *v
	dst := make([]byte, 0, len(src)*2)
	d := NewDecoderFromBytes(src)
	defer PutDecoder(d)
	depth := 0
	needNewline := false
	prevKind := Invalid

	for {
		tok, err := d.ReadToken()
		if err == io.EOF && prevKind != Invalid {
			break
		}
		if err != nil {
			return err
		}
		if d.sm.depth == 0 && prevKind != Invalid && (tok.Kind() != EndObject && tok.Kind() != EndArray) {
			return d.syntaxError("invalid data after top-level value")
		}

		k := tok.Kind()

		if k == EndObject || k == EndArray {
			depth--
			if prevKind != BeginObject && prevKind != BeginArray {
				dst = append(dst, '\n')
				dst = appendIndent(dst, prefix, indent, depth)
			}
			dst = append(dst, byte(k))
			prevKind = k
			needNewline = false
			continue
		}

		if needNewline {
			dst = append(dst, ',', '\n')
			dst = appendIndent(dst, prefix, indent, depth)
		} else if prevKind == BeginObject || prevKind == BeginArray {
			dst = append(dst, '\n')
			dst = appendIndent(dst, prefix, indent, depth)
		}

		switch k {
		case BeginObject, BeginArray:
			dst = append(dst, byte(k))
			depth++
			needNewline = false
		case String:
			dst = appendQuotedString(dst, tok.Str())
			if d.sm.last.needObjectValue() {
				dst = append(dst, ':', ' ')
				needNewline = false
			} else {
				needNewline = true
			}
		case Number:
			dst = append(dst, tok.Str()...)
			needNewline = true
		case Null:
			dst = append(dst, "null"...)
			needNewline = true
		case True:
			dst = append(dst, "true"...)
			needNewline = true
		case False:
			dst = append(dst, "false"...)
			needNewline = true
		default:
			needNewline = true
		}

		prevKind = k
	}
	dst = append(dst, '\n')
	*v = dst
	return nil
}

func appendIndent(dst []byte, prefix, indent string, depth int) []byte {
	dst = append(dst, prefix...)
	for i := 0; i < depth; i++ {
		dst = append(dst, indent...)
	}
	return dst
}

func (v Value) MarshalJSON() ([]byte, error) {
	return bytes.Clone([]byte(v)), nil
}

func (v *Value) UnmarshalJSON(b []byte) error {
	*v = bytes.Clone(b)
	return nil
}
