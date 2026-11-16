package jsontext

import (
	"io"
	"strings"
	"sync"
)

// Encoder writes JSON tokens to an output stream.
type Encoder struct {
	buf       []byte
	wr        io.Writer
	sm        stateMachine
	err       error
	indent    string
	prefix    string
	multiline bool
	escHTML   bool
	borrowed  bool
}

var encoderPool = sync.Pool{
	New: func() any { return &Encoder{} },
}

// NewEncoder creates a new streaming Encoder writing to w.
func NewEncoder(w io.Writer) *Encoder {
	e := encoderPool.Get().(*Encoder)
	e.Reset(w)
	return e
}

// NewEncoderToBytes creates a new Encoder that writes to a byte buffer.
func NewEncoderToBytes(buf *[]byte) *Encoder {
	e := encoderPool.Get().(*Encoder)
	if *buf == nil {
		*buf = make([]byte, 0, 256)
	}
	e.buf = (*buf)[:0]
	e.borrowed = true
	e.wr = nil
	e.err = nil
	e.sm.init()
	e.indent = ""
	e.prefix = ""
	e.multiline = false
	e.escHTML = false
	return e
}

// PutEncoder returns an Encoder to the pool for reuse.
func PutEncoder(e *Encoder) {
	if e.borrowed {
		e.buf = nil
	} else {
		e.buf = e.buf[:0]
	}
	e.wr = nil
	e.err = nil
	encoderPool.Put(e)
}

// WrapBuffer creates an Encoder that appends to an existing buffer.
func WrapBuffer(buf []byte) *Encoder {
	e := encoderPool.Get().(*Encoder)
	e.buf = buf
	e.borrowed = true
	e.wr = nil
	e.err = nil
	e.sm.init()
	e.indent = ""
	e.prefix = ""
	e.multiline = false
	e.escHTML = false
	return e
}

// AppendQuoteString appends the JSON-quoted form of s to dst.
func AppendQuoteString(dst []byte, s string) []byte {
	return appendQuotedString(dst, s)
}

// AppendQuoteStringHTML appends the HTML-safe JSON-quoted form of s to dst.
func AppendQuoteStringHTML(dst []byte, s string) []byte {
	return appendQuotedStringHTML(dst, s)
}

// Reset resets the encoder to write to a new writer.
func (e *Encoder) Reset(w io.Writer) {
	if cap(e.buf) < 256 || e.borrowed {
		e.buf = make([]byte, 0, 256)
	} else {
		e.buf = e.buf[:0]
	}
	e.borrowed = false
	e.wr = w
	e.err = nil
	e.sm.init()
	e.indent = ""
	e.prefix = ""
	e.multiline = false
	e.escHTML = false
}

// SetIndent sets the indentation for pretty-printing.
func (e *Encoder) SetIndent(prefix, indent string) {
	e.prefix = prefix
	e.indent = indent
	e.multiline = indent != "" || prefix != ""
}

// SetEscapeHTML configures whether HTML characters are escaped.
func (e *Encoder) SetEscapeHTML(on bool) {
	e.escHTML = on
}

// Bytes returns the accumulated output buffer.
func (e *Encoder) Bytes() []byte {
	return e.buf
}

// Flush writes any buffered data to the underlying writer.
func (e *Encoder) Flush() error {
	if e.wr == nil || len(e.buf) == 0 {
		return nil
	}
	_, err := e.wr.Write(e.buf)
	e.buf = e.buf[:0]
	if err != nil {
		e.err = err
	}
	return err
}

// WriteToken writes a single JSON token to the output.
func (e *Encoder) WriteToken(t Token) error {
	if e.err != nil {
		return e.err
	}

	k := t.Kind()

	if k != EndObject && k != EndArray {
		delim := e.sm.needDelim(k)
		if delim != 0 {
			e.buf = append(e.buf, delim)
		}
		if e.multiline && e.sm.depth > 0 && delim != ':' {
			e.buf = append(e.buf, '\n')
			e.writeIndent()
		} else if delim == ':' {
			if e.multiline {
				e.buf = append(e.buf, ' ')
			}
		}
	}

	switch k {
	case Null:
		e.buf = append(e.buf, "null"...)
		e.sm.appendValue()
	case True:
		e.buf = append(e.buf, "true"...)
		e.sm.appendValue()
	case False:
		e.buf = append(e.buf, "false"...)
		e.sm.appendValue()
	case String:
		if e.sm.last.needObjectName() {
			e.buf = e.appendString(e.buf, t.Str())
			e.sm.appendName()
		} else {
			e.buf = e.appendString(e.buf, t.Str())
			e.sm.appendValue()
		}
	case Number:
		e.buf = append(e.buf, t.Str()...)
		e.sm.appendValue()
	case BeginObject:
		e.buf = append(e.buf, '{')
		e.sm.pushObject()
	case EndObject:
		if e.multiline && e.sm.last.length() > 0 {
			e.buf = append(e.buf, '\n')
			e.sm.popObject()
			e.writeIndent()
		} else {
			e.sm.popObject()
		}
		e.buf = append(e.buf, '}')
	case BeginArray:
		e.buf = append(e.buf, '[')
		e.sm.pushArray()
	case EndArray:
		if e.multiline && e.sm.last.length() > 0 {
			e.buf = append(e.buf, '\n')
			e.sm.popArray()
			e.writeIndent()
		} else {
			e.sm.popArray()
		}
		e.buf = append(e.buf, ']')
	}

	if e.wr != nil && len(e.buf) > cap(e.buf)*3/4 {
		return e.Flush()
	}
	return nil
}

// WriteValue writes a complete JSON value to the output.
func (e *Encoder) WriteValue(v Value) error {
	if e.err != nil {
		return e.err
	}

	k := v.Kind()
	delim := e.sm.needDelim(k)
	if delim != 0 {
		e.buf = append(e.buf, delim)
	}
	if e.multiline && e.sm.depth > 0 && delim != ':' {
		e.buf = append(e.buf, '\n')
		e.writeIndent()
	} else if delim == ':' && e.multiline {
		e.buf = append(e.buf, ' ')
	}

	e.buf = append(e.buf, v...)

	if e.sm.last.needObjectName() {
		e.sm.appendName()
	} else {
		e.sm.appendValue()
	}

	if e.wr != nil && len(e.buf) > cap(e.buf)*3/4 {
		return e.Flush()
	}
	return nil
}

// WriteRaw writes raw bytes directly to the output buffer.
func (e *Encoder) WriteRaw(b []byte) {
	e.buf = append(e.buf, b...)
}

// WriteNull writes a JSON null value.
func (e *Encoder) WriteNull() error {
	if e.err != nil {
		return e.err
	}
	e.writeDelim(Null)
	e.buf = append(e.buf, "null"...)
	e.sm.appendValue()
	return e.maybeFlush()
}

// WriteBool writes a JSON boolean value.
func (e *Encoder) WriteBool(b bool) error {
	if e.err != nil {
		return e.err
	}
	if b {
		e.writeDelim(True)
		e.buf = append(e.buf, "true"...)
	} else {
		e.writeDelim(False)
		e.buf = append(e.buf, "false"...)
	}
	e.sm.appendValue()
	return e.maybeFlush()
}

// WriteString writes a JSON string value.
func (e *Encoder) WriteString(s string) error {
	if e.err != nil {
		return e.err
	}
	e.writeDelim(String)
	e.buf = e.appendString(e.buf, s)
	if e.sm.last.needObjectName() {
		e.sm.appendName()
	} else {
		e.sm.appendValue()
	}
	return e.maybeFlush()
}

// WriteInt writes a JSON integer value.
func (e *Encoder) WriteInt(n int64) error {
	if e.err != nil {
		return e.err
	}
	e.writeDelim(Number)
	e.buf = appendInt64(e.buf, n)
	e.sm.appendValue()
	return e.maybeFlush()
}

// WriteUint writes a JSON unsigned integer value.
func (e *Encoder) WriteUint(n uint64) error {
	if e.err != nil {
		return e.err
	}
	e.writeDelim(Number)
	e.buf = appendUint64(e.buf, n)
	e.sm.appendValue()
	return e.maybeFlush()
}

// WriteFloat writes a JSON floating-point value.
func (e *Encoder) WriteFloat(n float64) error {
	if e.err != nil {
		return e.err
	}
	e.writeDelim(Number)
	e.buf = FastAppendFloat(e.buf, n, 64)
	e.sm.appendValue()
	return e.maybeFlush()
}

func (e *Encoder) writeDelim(k Kind) {
	if k != EndObject && k != EndArray {
		delim := e.sm.needDelim(k)
		if delim != 0 {
			e.buf = append(e.buf, delim)
		}
		if e.multiline && e.sm.depth > 0 && delim != ':' {
			e.buf = append(e.buf, '\n')
			e.writeIndent()
		} else if delim == ':' && e.multiline {
			e.buf = append(e.buf, ' ')
		}
	}
}

func (e *Encoder) maybeFlush() error {
	if e.wr != nil && len(e.buf) > cap(e.buf)*3/4 {
		return e.Flush()
	}
	return nil
}

// OutputOffset returns the current output byte offset.
func (e *Encoder) OutputOffset() int64 {
	return int64(len(e.buf))
}

// AvailableBuffer returns a zero-length slice of the internal buffer for caller use.
func (e *Encoder) AvailableBuffer() []byte {
	if cap(e.buf)-len(e.buf) < 64 {
		newCap := cap(e.buf)*2 + 256
		newBuf := make([]byte, len(e.buf), newCap)
		copy(newBuf, e.buf)
		e.buf = newBuf
	}
	return e.buf[len(e.buf):len(e.buf)]
}

func (e *Encoder) appendString(dst []byte, s string) []byte {
	if e.escHTML {
		return appendQuotedStringHTML(dst, s)
	}
	return appendQuotedString(dst, s)
}

func (e *Encoder) writeIndent() {
	e.buf = append(e.buf, e.prefix...)
	for i := 0; i < e.sm.Depth(); i++ {
		e.buf = append(e.buf, e.indent...)
	}
}

const (
	lineSeparatorLead = "\xe2\x80"
	lineSeparatorLen  = len("\xe2\x80\xa8")
	lineSeparatorLast = 0xA8
	paraSeparatorLast = 0xA9
)

func indexLineSeparator(s string) int {
	for off := 0; ; {
		k := strings.Index(s[off:], lineSeparatorLead)
		if k < 0 {
			return -1
		}
		k += off
		if k+2 < len(s) && (s[k+2] == lineSeparatorLast || s[k+2] == paraSeparatorLast) {
			return k
		}
		off = k + 1
	}
}

func appendQuotedStringHTML(dst []byte, s string) []byte {
	dst = append(dst, '"')
	for {
		k := indexLineSeparator(s)
		if k < 0 {
			break
		}
		dst = appendHTMLEscapedBody(dst, s[:k])
		if s[k+2] == lineSeparatorLast {
			dst = append(dst, '\\', 'u', '2', '0', '2', '8')
		} else {
			dst = append(dst, '\\', 'u', '2', '0', '2', '9')
		}
		s = s[k+lineSeparatorLen:]
	}
	dst = appendHTMLEscapedBody(dst, s)
	return append(dst, '"')
}

func appendHTMLEscapedBody(dst []byte, s string) []byte {
	b := unsafeBytesFromString(s)
	n := FindSafeStringBytesHTML(b)
	if n == len(s) {
		return append(dst, s...)
	}
	start := 0
	i := n
	for i < len(s) {
		c := s[i]
		if c >= 0x20 && c != '"' && c != '\\' && c != '<' && c != '>' && c != '&' {
			i++
			continue
		}
		dst = append(dst, s[start:i]...)
		switch c {
		case '"':
			dst = append(dst, '\\', '"')
		case '\\':
			dst = append(dst, '\\', '\\')
		case '<':
			dst = append(dst, '\\', 'u', '0', '0', '3', 'c')
		case '>':
			dst = append(dst, '\\', 'u', '0', '0', '3', 'e')
		case '&':
			dst = append(dst, '\\', 'u', '0', '0', '2', '6')
		case '\b':
			dst = append(dst, '\\', 'b')
		case '\f':
			dst = append(dst, '\\', 'f')
		case '\n':
			dst = append(dst, '\\', 'n')
		case '\r':
			dst = append(dst, '\\', 'r')
		case '\t':
			dst = append(dst, '\\', 't')
		default:
			dst = append(dst, '\\', 'u', '0', '0', hexChar(c>>4), hexChar(c&0xF))
		}
		i++
		start = i
		if i < len(s) {
			rem := unsafeBytesFromString(s[i:])
			skip := FindSafeStringBytesHTML(rem)
			i += skip
		}
	}
	return append(dst, s[start:]...)
}

// AppendInt appends a JSON integer to dst.
func AppendInt(dst []byte, n int64) []byte {
	return appendInt64(dst, n)
}

// AppendUint appends a JSON unsigned integer to dst.
func AppendUint(dst []byte, n uint64) []byte {
	return appendUint64(dst, n)
}

// AppendFloat appends a JSON floating-point number to dst.
func AppendFloat(dst []byte, n float64, bits int) []byte {
	return FastAppendFloat(dst, n, bits)
}
