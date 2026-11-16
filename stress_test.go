package json_test

import (
	"bytes"
	"fmt"
	"os"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	alosjson "github.com/guno1928/alos-json"
)

const (
	stressWorkers    = 16
	stressIterations = 300
	scalingEnv       = "ALOS_SCALING"
	scalingFactor    = 8
	maxScalingRatio  = 3 * scalingFactor
)

type stressCase struct {
	doc     []byte
	valid   bool
	decoded any
	encoded []byte
	record  guardRecordPortable
	recOK   bool
}

type guardRecordPortable struct {
	ID       int64                  `json:"id"`
	Name     string                 `json:"name"`
	Weight   float64                `json:"weight"`
	Tags     []string               `json:"tags"`
	Ints     []int64                `json:"ints"`
	Floats   []float64              `json:"floats"`
	Children []*guardRecordPortable `json:"children"`
	Extra    map[string]any         `json:"extra"`
}

func buildStressCases(t *testing.T) []stressCase {
	g := &validDocGen{x: 0x1B873593CC9E2D51}
	docs := []string{
		`{"id":7,"name":"` + strings.Repeat(`\u00e9x\"`, 400) + `","weight":0.1,"tags":["a","` + strings.Repeat("long", 300) + `"],"ints":[1,2,3],"floats":[1.5,-2.25e-8],"children":[{"id":8,"children":[]}],"extra":{"k":[1,"v",null]}}`,
		`{"ints":[` + strings.Repeat("123456789,", 500) + `0],"floats":[` + strings.Repeat("-72.98765432101234,", 300) + `1]}`,
	}
	for range 60 {
		var sb strings.Builder
		g.value(&sb, 0)
		docs = append(docs, sb.String())
		docs = append(docs, string(g.mutate([]byte(sb.String()))))
	}
	cases := make([]stressCase, 0, len(docs))
	for _, d := range docs {
		c := stressCase{doc: []byte(d), valid: alosjson.Valid([]byte(d))}
		if c.valid {
			if err := alosjson.Unmarshal(c.doc, &c.decoded); err != nil {
				t.Fatalf("Unmarshal(%q): %v", d, err)
			}
			out, err := alosjson.Marshal(c.decoded, alosjson.Deterministic(true))
			if err != nil {
				t.Fatalf("Marshal: %v", err)
			}
			c.encoded = out
			c.recOK = alosjson.Unmarshal(c.doc, &c.record) == nil
		}
		cases = append(cases, c)
	}
	return cases
}

func TestConcurrentResultsAreIndependent(t *testing.T) {
	cases := buildStressCases(t)
	var wg sync.WaitGroup
	errs := make(chan error, stressWorkers)
	for w := range stressWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			type keptResult struct {
				value any
				want  *stressCase
			}
			kept := make([]keptResult, 0, stressIterations)
			for it := range stressIterations {
				c := &cases[(w*31+it*7)%len(cases)]
				if alosjson.Valid(c.doc) != c.valid {
					errs <- fmt.Errorf("Valid changed for %q", c.doc)
					return
				}
				if !c.valid {
					continue
				}
				var got any
				if err := alosjson.Unmarshal(c.doc, &got); err != nil || !reflect.DeepEqual(got, c.decoded) {
					errs <- fmt.Errorf("Unmarshal changed for %q: %v", c.doc, err)
					return
				}
				kept = append(kept, keptResult{value: got, want: c})
				out, err := alosjson.Marshal(got, alosjson.Deterministic(true))
				if err != nil || !bytes.Equal(out, c.encoded) {
					errs <- fmt.Errorf("Marshal changed for %q: %v", c.doc, err)
					return
				}
				var rec guardRecordPortable
				if ok := alosjson.Unmarshal(c.doc, &rec) == nil; ok != c.recOK || ok && !reflect.DeepEqual(rec, c.record) {
					errs <- fmt.Errorf("typed Unmarshal changed for %q", c.doc)
					return
				}
			}
			for i, k := range kept {
				if !reflect.DeepEqual(k.value, k.want.decoded) {
					errs <- fmt.Errorf("kept value %d was overwritten by a later decode", i)
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func adversarialInputs(n int) map[string][]byte {
	return map[string][]byte{
		"backslash-run-unterminated": []byte(`"` + strings.Repeat(`\\`, n)),
		"escapes-error-at-end":       []byte(`["` + strings.Repeat(`A\n`, n) + `\x"]`),
		"escapes-valid":              []byte(`["` + strings.Repeat(`A\n\"`, n) + `"]`),
		"many-short-strings":         []byte(`[` + strings.Repeat(`"a",`, n) + `"b"]`),
		"long-number":                []byte(strings.Repeat("1", n*4)),
		"long-fraction":              []byte("0." + strings.Repeat("7", n*4)),
		"nested-at-limit-repeated":   []byte("[" + strings.Repeat(strings.Repeat("[", 9000)+strings.Repeat("]", 9000)+",", max(1, n/4000)) + "0]"),
		"wide-object":                []byte("{" + strings.Repeat(`"k":1,`, n) + `"z":2}`),
		"unclosed-arrays":            []byte(strings.Repeat("[1,", n)),
		"whitespace":                 []byte("[" + strings.Repeat(" \n\t\r", n) + "1]"),
	}
}

func timeIt(f func()) time.Duration {
	best := time.Duration(1 << 62)
	for range 5 {
		start := time.Now()
		f()
		best = min(best, time.Since(start))
	}
	return best
}

func TestAdversarialInputsScaleLinearly(t *testing.T) {
	if os.Getenv(scalingEnv) == "" {
		t.Skip(scalingEnv + " not set")
	}
	const base = 20_000
	small, large := adversarialInputs(base), adversarialInputs(base*scalingFactor)
	for name := range small {
		for _, op := range []struct {
			label string
			run   func([]byte)
		}{
			{"Valid", func(b []byte) { alosjson.Valid(b) }},
			{"Unmarshal", func(b []byte) {
				var v any
				alosjson.Unmarshal(b, &v)
			}},
		} {
			s := timeIt(func() { op.run(small[name]) })
			l := timeIt(func() { op.run(large[name]) })
			ratio := float64(l) / float64(max(s, time.Microsecond))
			t.Logf("%-28s %-9s %8d bytes %10v | %9d bytes %10v | ratio %.1f", name, op.label, len(small[name]), s, len(large[name]), l, ratio)
			if ratio > maxScalingRatio {
				t.Errorf("%s/%s grows superlinearly: ratio %.1f for %dx input", name, op.label, ratio, scalingFactor)
			}
		}
	}
}
