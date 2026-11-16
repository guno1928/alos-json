package jsontext

import (
	"io"
	"sync"

	"github.com/guno1928/alos-json/internal/scan"
)

const (
	defaultBufSize = 4096
	minRead        = 512

	maxNestingDepth = scan.MaxDepth

	maxConsecutiveEmptyReads = 100
)

// Decoder reads and decodes JSON tokens from an input stream.
type Decoder struct {
	buf        []byte
	pos        int
	baseOffset int64
	rd         io.Reader
	streaming  bool
	prepared   bool
	sm         stateMachine
	err        error
	serr       error
	intern     stringCache
}

var decoderPool = sync.Pool{
	New: func() any { return &Decoder{} },
}

// NewDecoder creates a new streaming Decoder reading from r.
func NewDecoder(r io.Reader) *Decoder {
	d := decoderPool.Get().(*Decoder)
	d.Reset(r)
	return d
}

// NewDecoderFromBytes creates a new Decoder reading from a byte slice.
func NewDecoderFromBytes(b []byte) *Decoder {
	d := decoderPool.Get().(*Decoder)
	d.buf = b
	d.pos = 0
	d.baseOffset = 0
	d.rd = nil
	d.streaming = false
	d.resetState()
	return d
}

// PutDecoder returns a Decoder to the pool for reuse.
func PutDecoder(d *Decoder) {
	if d.streaming {
		d.buf = d.buf[:0]
	} else {
		d.buf = nil
	}
	d.rd = nil
	d.err = nil
	d.serr = nil
	d.pos = 0
	decoderPool.Put(d)
}

// Reset resets the decoder to read from a new reader.
func (d *Decoder) Reset(r io.Reader) {
	if cap(d.buf) < defaultBufSize || !d.streaming {
		d.buf = make([]byte, 0, defaultBufSize)
	} else {
		d.buf = d.buf[:0]
	}
	d.pos = 0
	d.baseOffset = 0
	d.rd = r
	d.streaming = true
	d.resetState()
}

func (d *Decoder) resetState() {
	d.err = nil
	d.serr = nil
	d.prepared = false
	d.sm.init()
}

// InputOffset returns the byte offset of the current position.
func (d *Decoder) InputOffset() int64 {
	return d.baseOffset + int64(d.pos)
}

// StackDepth returns the current nesting depth.
func (d *Decoder) StackDepth() int {
	return d.sm.Depth()
}

func (d *Decoder) syntaxError(msg string) error {
	d.serr = newSyntacticError(d.InputOffset(), msg)
	return d.serr
}

func (d *Decoder) canFill() bool {
	return d.streaming && d.rd != nil && d.err == nil
}

func (d *Decoder) eofError() error {
	if d.err != nil && d.err != io.EOF {
		return d.err
	}
	if d.sm.depth > 0 {
		return io.ErrUnexpectedEOF
	}
	return io.EOF
}

func (d *Decoder) nextNonSpace() (byte, error) {
	for {
		buf := d.buf
		pos := d.pos
		for pos < len(buf) {
			c := buf[pos]
			if !wsTable[c] {
				d.pos = pos
				return c, nil
			}
			pos++
		}
		d.pos = pos
		if !d.canFill() {
			return 0, d.eofError()
		}
		d.fill()
	}
}

func (d *Decoder) prepare() (byte, error) {
	if d.serr != nil {
		return 0, d.serr
	}
	if d.prepared {
		return d.buf[d.pos], nil
	}
	c, err := d.nextNonSpace()
	if err != nil {
		return 0, err
	}
	if d.sm.depth > 0 {
		e := d.sm.last
		switch {
		case e.needObjectValue():
			if c != ':' {
				return 0, d.syntaxError("missing ':' after object name")
			}
			d.pos++
			if c, err = d.nextNonSpace(); err != nil {
				return 0, d.unexpectedEOF(err)
			}
		case e.length() > 0:
			if c == ',' {
				d.pos++
				if c, err = d.nextNonSpace(); err != nil {
					return 0, d.unexpectedEOF(err)
				}
				if c == ']' || c == '}' {
					return 0, d.syntaxError("trailing comma before end of container")
				}
			} else if c != ']' && c != '}' {
				return 0, d.syntaxError("missing ',' between elements")
			}
		}
		if e.needObjectName() && c != '"' && c != '}' {
			return 0, d.syntaxError("expected string for object name")
		}
	}
	d.prepared = true
	return c, nil
}

func (d *Decoder) unexpectedEOF(err error) error {
	if err == io.EOF {
		return io.ErrUnexpectedEOF
	}
	return err
}

// PeekKind returns the kind of the next token without consuming it.
func (d *Decoder) PeekKind() Kind {
	c, err := d.prepare()
	if err != nil {
		return Invalid
	}
	return normalizeKind(c)
}

func (d *Decoder) consumed(n int) {
	d.pos += n
	d.prepared = false
}

func (d *Decoder) scalarLen(k Kind) int {
	for {
		rest := d.buf[d.pos:]
		var n int
		switch k {
		case String:
			n = consumeString(rest)
		case Number:
			n = consumeNumber(rest)
			if n == len(rest) && d.canFill() {
				n = 0
			}
		case Null:
			n = consumeNull(rest)
		case True:
			n = consumeTrue(rest)
		case False:
			n = consumeFalse(rest)
		}
		if n > 0 || !d.canFill() {
			return n
		}
		d.fill()
	}
}

func (d *Decoder) valueLen() int {
	for {
		rest := d.buf[d.pos:]
		n := consumeValueDepth(rest, d.sm.depth)
		if n > 0 && n < len(rest) || !d.canFill() {
			return n
		}
		d.fill()
	}
}

// ReadToken reads and returns the next JSON token.
func (d *Decoder) ReadToken() (Token, error) {
	c, err := d.prepare()
	if err != nil {
		return Token{}, err
	}
	k := normalizeKind(c)
	switch k {
	case BeginObject:
		if d.sm.depth >= maxNestingDepth {
			return Token{}, d.syntaxError("exceeded max nesting depth")
		}
		d.consumed(1)
		d.sm.pushObject()
		return TBeginObject, nil

	case EndObject:
		if !d.sm.popObject() {
			return Token{}, d.syntaxError("unexpected '}'")
		}
		d.consumed(1)
		return TEndObject, nil

	case BeginArray:
		if d.sm.depth >= maxNestingDepth {
			return Token{}, d.syntaxError("exceeded max nesting depth")
		}
		d.consumed(1)
		d.sm.pushArray()
		return TBeginArray, nil

	case EndArray:
		if !d.sm.popArray() {
			return Token{}, d.syntaxError("unexpected ']'")
		}
		d.consumed(1)
		return TEndArray, nil

	case Null, True, False:
		if d.scalarLen(k) == 0 {
			return Token{}, d.syntaxError("invalid literal")
		}
		d.consumed(len(literalToken(k).raw))
		d.sm.appendValue()
		return literalToken(k), nil

	case Number:
		n := d.scalarLen(Number)
		if n == 0 {
			return Token{}, d.syntaxError("invalid number")
		}
		raw := d.buf[d.pos : d.pos+n]
		tok := Token{k: Number, raw: raw}
		d.consumed(n)
		d.sm.appendValue()
		return tok, nil

	case String:
		n := d.scalarLen(String)
		if n == 0 {
			return Token{}, d.syntaxError("invalid string")
		}
		raw := d.buf[d.pos : d.pos+n]
		tok := Token{k: String, raw: raw, str: d.unquoteOwned(raw)}
		d.consumed(n)
		d.markString()
		return tok, nil

	default:
		return Token{}, d.syntaxError("unexpected character")
	}
}

func literalToken(k Kind) Token {
	switch k {
	case Null:
		return TNull
	case True:
		return TTrue
	default:
		return TFalse
	}
}

func (d *Decoder) markString() {
	if d.sm.last.needObjectName() {
		d.sm.appendName()
	} else {
		d.sm.appendValue()
	}
}

// ReadValue reads a complete JSON value as raw bytes.
// The returned Value is only valid until the next call on the Decoder.
func (d *Decoder) ReadValue() (Value, error) {
	c, err := d.prepare()
	if err != nil {
		return nil, err
	}
	k := normalizeKind(c)
	if k == EndObject || k == EndArray || k == Invalid {
		return nil, d.syntaxError("invalid JSON value")
	}
	n := d.valueLen()
	if n == 0 {
		return nil, d.syntaxError("invalid JSON value")
	}
	val := Value(d.buf[d.pos : d.pos+n])
	d.consumed(n)
	if k == String {
		d.markString()
	} else {
		d.sm.appendValue()
	}
	return val, nil
}

// SkipValue skips over the next JSON value.
func (d *Decoder) SkipValue() error {
	_, err := d.ReadValue()
	return err
}

// More reports whether there is another element in the current array or object.
func (d *Decoder) More() bool {
	c, err := d.prepare()
	if err != nil {
		return false
	}
	return c != ']' && c != '}'
}

func (d *Decoder) fill() {
	if d.err != nil {
		return
	}
	if d.pos > 0 {
		d.baseOffset += int64(d.pos)
		n := copy(d.buf, d.buf[d.pos:])
		d.buf = d.buf[:n]
		d.pos = 0
	}
	if cap(d.buf)-len(d.buf) < minRead {
		newBuf := make([]byte, len(d.buf), cap(d.buf)*2+minRead)
		copy(newBuf, d.buf)
		d.buf = newBuf
	}
	for range maxConsecutiveEmptyReads {
		n, err := d.rd.Read(d.buf[len(d.buf):cap(d.buf)])
		d.buf = d.buf[:len(d.buf)+n]
		if err != nil {
			d.err = err
			return
		}
		if n > 0 {
			return
		}
	}
	d.err = io.ErrNoProgress
}

// ReadStringFast reads a string token and returns the unquoted content.
// Returns empty string and false if the next token is not a string.
func (d *Decoder) ReadStringFast() (string, bool) {
	c, err := d.prepare()
	if err != nil || c != '"' {
		return "", false
	}
	n := d.scalarLen(String)
	if n == 0 {
		return "", false
	}
	s := d.unquoteOwned(d.buf[d.pos : d.pos+n])
	d.consumed(n)
	d.markString()
	return s, true
}

// ReadNumberStr reads a number token and returns the raw string.
// The returned string is only valid until the next call on the Decoder.
func (d *Decoder) ReadNumberStr() (string, bool) {
	c, err := d.prepare()
	if err != nil || normalizeKind(c) != Number {
		return "", false
	}
	n := d.scalarLen(Number)
	if n == 0 {
		return "", false
	}
	s := unsafeStringFromBytes(d.buf[d.pos : d.pos+n])
	d.consumed(n)
	d.sm.appendValue()
	return s, true
}

func (d *Decoder) unquoteOwned(raw []byte) string {
	inner := raw[1 : len(raw)-1]
	if !hasEscapeFast(inner) {
		return d.intern.owned(inner)
	}
	var buf [256]byte
	return d.intern.owned(appendUnquote(buf[:0], raw))
}

func hasEscapeFast(b []byte) bool {
	if len(b) == 0 {
		return false
	}
	return scan.IndexBackslash(b) < len(b)
}
