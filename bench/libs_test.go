package bench

import (
	stdjson "encoding/json"

	"github.com/bytedance/sonic"
	goccy "github.com/goccy/go-json"
	alosjson "github.com/guno1928/alos-json"
	jsoniter "github.com/json-iterator/go"
	segment "github.com/segmentio/encoding/json"
)

type codec struct {
	name      string
	marshal   func(any) ([]byte, error)
	unmarshal func([]byte, any) error
	valid     func([]byte) bool
}

var codecs = []codec{
	{name: "alos", marshal: func(v any) ([]byte, error) { return alosjson.Marshal(v) }, unmarshal: func(b []byte, v any) error { return alosjson.Unmarshal(b, v) }, valid: alosjson.Valid},
	{name: "std_v1", marshal: stdjson.Marshal, unmarshal: stdjson.Unmarshal, valid: stdjson.Valid},
	{name: "sonic", marshal: sonic.Marshal, unmarshal: sonic.Unmarshal, valid: sonic.Valid},
	{name: "sonic_std", marshal: sonic.ConfigStd.Marshal, unmarshal: sonic.ConfigStd.Unmarshal, valid: sonic.ConfigStd.Valid},
	{name: "goccy", marshal: goccy.Marshal, unmarshal: goccy.Unmarshal, valid: goccy.Valid},
	{name: "jsoniter", marshal: jsoniter.Marshal, unmarshal: jsoniter.Unmarshal, valid: jsoniter.Valid},
	{name: "segmentio", marshal: segment.Marshal, unmarshal: segment.Unmarshal, valid: segment.Valid},
}
