package json

import (
	"fmt"
	"reflect"

	"github.com/guno1928/alos-json/jsontext"
)

// SemanticError describes a semantic error during marshal or unmarshal.
type SemanticError struct {
	ByteOffset int64
	JSONKind   jsontext.Kind
	GoType     reflect.Type
	Msg        string
	Err        error
}

func (e *SemanticError) Error() string {
	s := "json: "
	if e.GoType != nil {
		s += fmt.Sprintf("type %v: ", e.GoType)
	}
	if e.Msg != "" {
		s += e.Msg
	}
	if e.Err != nil {
		if e.Msg != "" {
			s += ": "
		}
		s += e.Err.Error()
	}
	return s
}

func (e *SemanticError) Unwrap() error { return e.Err }

func newSemanticError(goType reflect.Type, msg string) error {
	return &SemanticError{GoType: goType, Msg: msg}
}

// ErrUnknownName is the sentinel error for unknown object member names.
var ErrUnknownName = fmt.Errorf("unknown object member name")
