package json

import (
	"bytes"
	"testing"
)

const spareSentinel = 0xAA

type spareInner struct {
	ID    int64   `json:"id"`
	Label string  `json:"label"`
	Ratio float64 `json:"ratio"`
}

type spareOuter struct {
	Name     string            `json:"name"`
	Count    int               `json:"count"`
	Items    []spareInner      `json:"items"`
	Ints     []int64           `json:"ints"`
	Floats   []float64         `json:"floats"`
	Words    []string          `json:"words"`
	Optional *spareInner       `json:"optional,omitempty"`
	Meta     map[string]string `json:"meta"`
}

func TestMarshalAppendLeavesSpareCapacityUntouched(t *testing.T) {
	values := []any{
		&spareOuter{
			Name:   "outer",
			Count:  3,
			Items:  []spareInner{{1, "a", 0.5}, {-22, "bb", 1e21}},
			Ints:   []int64{0, 7, -123456789012345, 99},
			Floats: []float64{0.1, -2.5e-8},
			Words:  []string{"x", "with \"quote\"", ""},
			Meta:   map[string]string{"k": "v"},
		},
		&spareInner{ID: 1, Label: "z", Ratio: 2},
		map[string]any{"a": 1.0, "b": []any{"c", true, nil}},
		[]int{1, 22, 333},
		"plain string",
	}
	for _, indent := range []bool{false, true} {
		for i, v := range values {
			backing := bytes.Repeat([]byte{spareSentinel}, 4096)
			dst := append(backing[:0], "prefix:"...)
			var opts []Option
			if indent {
				opts = append(opts, WithIndent("", "  "))
			}
			out, err := MarshalAppend(dst, v, opts...)
			if err != nil {
				t.Fatalf("value %d: %v", i, err)
			}
			if &out[0] != &backing[0] {
				t.Fatalf("value %d: output did not reuse dst backing array", i)
			}
			for j := len(out); j < len(backing); j++ {
				if backing[j] != spareSentinel {
					t.Fatalf("value %d indent=%v: byte %d past result (len %d) was overwritten with %#x", i, indent, j, len(out), backing[j])
				}
			}
		}
	}
}
