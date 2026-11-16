package jsontext

import "fmt"

type SyntacticError struct {
	ByteOffset int64
	Msg        string
	Err        error
}

func (e *SyntacticError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("jsontext: offset %d: %s: %v", e.ByteOffset, e.Msg, e.Err)
	}
	return fmt.Sprintf("jsontext: offset %d: %s", e.ByteOffset, e.Msg)
}

func (e *SyntacticError) Unwrap() error { return e.Err }

func newSyntacticError(offset int64, msg string) error {
	return &SyntacticError{ByteOffset: offset, Msg: msg}
}
