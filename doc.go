// Package json encodes and decodes JSON with an API shaped like
// encoding/json/v2: Marshal, Unmarshal and functional Options.
//
// It provides:
//
//   - Streaming token Encoder and Decoder in the jsontext subpackage
//   - Struct tags: json:"name,omitempty,omitzero,string,inline,nocase"
//   - MarshalerTo/UnmarshalerFrom, Marshaler/Unmarshaler and
//     encoding.TextMarshaler/TextUnmarshaler support
//   - Type-specific overrides with MarshalFunc, MarshalToFunc,
//     UnmarshalFunc and UnmarshalFromFunc
//   - Options for deterministic map order, HTML-safe escaping, indentation,
//     case-insensitive names, unknown-member rejection, omitting zero
//     struct fields and encoding numbers as strings
//   - []byte encoded as base64
package json
