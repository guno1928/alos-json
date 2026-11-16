package json

import (
	"github.com/guno1928/alos-json/jsontext"
)

// MarshalerTo is the v2-style marshal interface.
// Implementations write exactly one JSON value to the encoder.
type MarshalerTo interface {
	MarshalJSONTo(enc *jsontext.Encoder) error
}

// Marshaler is the v1-style marshal interface.
type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

// UnmarshalerFrom is the v2-style unmarshal interface.
// Implementations read exactly one JSON value from the decoder.
type UnmarshalerFrom interface {
	UnmarshalJSONFrom(dec *jsontext.Decoder) error
}

// Unmarshaler is the v1-style unmarshal interface.
type Unmarshaler interface {
	UnmarshalJSON([]byte) error
}

// IsZeroer is checked by omitzero to determine if a value is zero.
type IsZeroer interface {
	IsZero() bool
}
