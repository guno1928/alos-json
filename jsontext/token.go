package jsontext

import (
	"math"
	"strconv"
	"unsafe"
)

// Token represents a single JSON token.
type Token struct {
	raw []byte
	str string
	num uint64
	k   Kind
}

var (
	TNull        = Token{k: Null, raw: []byte("null")}
	TFalse       = Token{k: False, raw: []byte("false")}
	TTrue        = Token{k: True, raw: []byte("true")}
	TBeginObject = Token{k: BeginObject, raw: []byte("{")}
	TEndObject   = Token{k: EndObject, raw: []byte("}")}
	TBeginArray  = Token{k: BeginArray, raw: []byte("[")}
	TEndArray    = Token{k: EndArray, raw: []byte("]")}
)

// Bool creates a boolean Token.
func Bool(b bool) Token {
	if b {
		return TTrue
	}
	return TFalse
}

// Str creates a string Token.
func Str(s string) Token {
	return Token{k: String, str: s}
}

// Int creates an integer Token.
func Int(n int64) Token {
	return Token{k: Number, num: uint64(n), str: strconv.FormatInt(n, 10)}
}

// Uint creates an unsigned integer Token.
func Uint(n uint64) Token {
	return Token{k: Number, num: n, str: strconv.FormatUint(n, 10)}
}

// Float creates a floating-point Token.
func Float(n float64) Token {
	return Token{k: Number, num: math.Float64bits(n), str: strconv.FormatFloat(n, 'f', -1, 64)}
}

func (t Token) Kind() Kind  { return t.k }
func (t Token) Raw() []byte { return t.raw }

// Bool returns the boolean value of the token.
func (t Token) Bool() bool {
	return t.k == True
}

// Str returns the string value of the token.
func (t Token) Str() string {
	if t.k == Number && t.str == "" {
		return string(t.raw)
	}
	return t.str
}

// Int returns the int64 value of the token.
func (t Token) Int() int64 {
	return int64(t.num)
}

// Uint returns the uint64 value of the token.
func (t Token) Uint() uint64 {
	return t.num
}

// Float returns the float64 value of the token.
func (t Token) Float() float64 {
	return math.Float64frombits(t.num)
}

func (t Token) String() string {
	switch t.k {
	case Null:
		return "null"
	case False:
		return "false"
	case True:
		return "true"
	case String:
		return t.str
	case Number:
		return t.Str()
	case BeginObject:
		return "{"
	case EndObject:
		return "}"
	case BeginArray:
		return "["
	case EndArray:
		return "]"
	default:
		return ""
	}
}

// Clone returns a deep copy of the token.
func (t Token) Clone() Token {
	out := t
	if len(t.raw) > 0 {
		out.raw = make([]byte, len(t.raw))
		copy(out.raw, t.raw)
	}
	if t.k == String {
		out.str = string([]byte(t.str))
	}
	return out
}

func unsafeStringFromBytes(b []byte) string {
	return unsafe.String(unsafe.SliceData(b), len(b))
}

func unsafeBytesFromString(s string) []byte {
	return unsafe.Slice(unsafe.StringData(s), len(s))
}
