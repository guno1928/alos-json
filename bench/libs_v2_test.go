//go:build goexperiment.jsonv2

package bench

import (
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
)

func init() {
	codecs = append(codecs, codec{
		name:      "json_v2",
		marshal:   func(v any) ([]byte, error) { return jsonv2.Marshal(v) },
		unmarshal: func(b []byte, v any) error { return jsonv2.Unmarshal(b, v) },
		valid:     func(b []byte) bool { return jsontext.Value(b).IsValid() },
	})
}
