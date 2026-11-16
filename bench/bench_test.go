package bench

import (
	stdjson "encoding/json"
	"reflect"
	"testing"
)

var (
	sinkBytes []byte
	sinkErr   error
	sinkBool  bool
)

func runMarshal(c *codec, v any) {
	sinkBytes, sinkErr = c.marshal(v)
}

func runUnmarshal(c *codec, data []byte, newTarget func() any) {
	sinkErr = c.unmarshal(data, newTarget())
}

func runUnmarshalAny(c *codec, data []byte) {
	var v any
	sinkErr = c.unmarshal(data, &v)
}

func runValid(c *codec, data []byte) {
	sinkBool = c.valid(data)
}

func BenchmarkMarshal(b *testing.B) {
	for _, d := range datasets {
		for i := range codecs {
			c := &codecs[i]
			b.Run("data="+d.name+"/lib="+c.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(d.json)))
				for b.Loop() {
					runMarshal(c, d.value)
				}
			})
		}
	}
}

func BenchmarkUnmarshal(b *testing.B) {
	for _, d := range datasets {
		for i := range codecs {
			c := &codecs[i]
			b.Run("data="+d.name+"/lib="+c.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(d.json)))
				for b.Loop() {
					runUnmarshal(c, d.json, d.newTarget)
				}
			})
		}
	}
}

func BenchmarkMarshalAny(b *testing.B) {
	for _, d := range genericDatasets {
		for i := range codecs {
			c := &codecs[i]
			b.Run("data="+d.name+"/lib="+c.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(d.json)))
				for b.Loop() {
					runMarshal(c, d.value)
				}
			})
		}
	}
}

func BenchmarkUnmarshalAny(b *testing.B) {
	for _, d := range genericDatasets {
		for i := range codecs {
			c := &codecs[i]
			b.Run("data="+d.name+"/lib="+c.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(d.json)))
				for b.Loop() {
					runUnmarshalAny(c, d.json)
				}
			})
		}
	}
}

func BenchmarkValid(b *testing.B) {
	for _, d := range datasets {
		for i := range codecs {
			c := &codecs[i]
			b.Run("data="+d.name+"/lib="+c.name, func(b *testing.B) {
				b.ReportAllocs()
				b.SetBytes(int64(len(d.json)))
				for b.Loop() {
					runValid(c, d.json)
				}
			})
		}
	}
}

func TestCodecsAgree(t *testing.T) {
	for _, d := range datasets {
		want := d.newTarget()
		if err := stdjson.Unmarshal(d.json, want); err != nil {
			t.Fatalf("%s: std unmarshal: %v", d.name, err)
		}
		for i := range codecs {
			c := &codecs[i]
			got := d.newTarget()
			if err := c.unmarshal(d.json, got); err != nil {
				t.Errorf("%s/%s: unmarshal error: %v", d.name, c.name, err)
			} else if !reflect.DeepEqual(got, want) {
				t.Errorf("%s/%s: unmarshal result differs from encoding/json", d.name, c.name)
			}
			out, err := c.marshal(d.value)
			if err != nil {
				t.Errorf("%s/%s: marshal error: %v", d.name, c.name, err)
				continue
			}
			if !c.valid(out) || !stdjson.Valid(out) {
				t.Errorf("%s/%s: marshal produced invalid JSON", d.name, c.name)
				continue
			}
			back := d.newTarget()
			if err := stdjson.Unmarshal(out, back); err != nil {
				t.Errorf("%s/%s: std cannot decode marshal output: %v", d.name, c.name, err)
			} else if !reflect.DeepEqual(back, want) {
				t.Errorf("%s/%s: marshal output decodes to a different value", d.name, c.name)
			}
			if !c.valid(d.json) {
				t.Errorf("%s/%s: Valid rejected valid input", d.name, c.name)
			}
		}
	}
	for _, d := range genericDatasets {
		for i := range codecs {
			c := &codecs[i]
			var got any
			if err := c.unmarshal(d.json, &got); err != nil {
				t.Errorf("any/%s/%s: unmarshal error: %v", d.name, c.name, err)
			} else if !reflect.DeepEqual(got, d.value) {
				t.Errorf("any/%s/%s: unmarshal-any result differs from encoding/json", d.name, c.name)
			}
			out, err := c.marshal(d.value)
			if err != nil {
				t.Errorf("any/%s/%s: marshal error: %v", d.name, c.name, err)
				continue
			}
			var back any
			if err := stdjson.Unmarshal(out, &back); err != nil {
				t.Errorf("any/%s/%s: std cannot decode marshal-any output: %v", d.name, c.name, err)
			} else if !reflect.DeepEqual(back, d.value) {
				t.Errorf("any/%s/%s: marshal-any output decodes to a different value", d.name, c.name)
			}
		}
	}
}

func TestDatasetSizes(t *testing.T) {
	for _, d := range datasets {
		t.Logf("%-18s %9d bytes", d.name, len(d.json))
	}
}
