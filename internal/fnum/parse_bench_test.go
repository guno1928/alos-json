package fnum

import "testing"

var (
	parseBenchInputs = map[string][]string{
		"short":  {"98.5", "50.1", "7.25", "123.4", "0.5"},
		"medium": {"139.6917", "35.6895", "12.345678"},
		"long":   {"-72.98765432109877", "-71.8641975308643", "12.345678901234567"},
	}
	parseBenchSink float64
)

func parseAll(in []string) float64 {
	total := 0.0
	for _, s := range in {
		v, _ := ParseFloat64(s)
		total += v
	}
	return total
}

func BenchmarkParseFloat64(b *testing.B) {
	for _, name := range []string{"short", "medium", "long"} {
		in := parseBenchInputs[name]
		b.Run(name, func(b *testing.B) {
			for b.Loop() {
				parseBenchSink += parseAll(in)
			}
		})
	}
}
