package json_test

import (
	stdjson "encoding/json"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"testing"

	alosjson "github.com/guno1928/alos-json"
)

const (
	slabTestDocs      = 400
	slabTestDocValues = 37
	slabTestWorkers   = 8
)

func slabTestDoc(seed int) (string, []float64) {
	var b strings.Builder
	want := make([]float64, slabTestDocValues)
	b.WriteByte('[')
	for i := range want {
		want[i] = float64(seed*1000+i) + 0.25
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(want[i], 'f', -1, 64))
	}
	b.WriteByte(']')
	return b.String(), want
}

func decodeAnyFloats(t *testing.T, doc string) []any {
	t.Helper()
	var v any
	if err := alosjson.Unmarshal([]byte(doc), &v); err != nil {
		t.Fatal(err)
	}
	return v.([]any)
}

func checkAnyFloats(t *testing.T, got []any, want []float64) {
	t.Helper()
	for i, w := range want {
		f, ok := got[i].(float64)
		if !ok || f != w || got[i] != any(w) || reflect.ValueOf(got[i]).Float() != w {
			t.Fatalf("value %d = %#v, want %v", i, got[i], w)
		}
	}
}

func TestAnyFloatsSurviveLaterDecodes(t *testing.T) {
	kept := make([][]any, slabTestDocs)
	wants := make([][]float64, slabTestDocs)
	for i := range kept {
		doc, want := slabTestDoc(i)
		kept[i], wants[i] = decodeAnyFloats(t, doc), want
		if i%50 == 0 {
			runtime.GC()
		}
	}
	runtime.GC()
	for i := range kept {
		checkAnyFloats(t, kept[i], wants[i])
	}
	kept[0][0] = 99.5
	doc, want := slabTestDoc(1)
	checkAnyFloats(t, decodeAnyFloats(t, doc), want)
	checkAnyFloats(t, kept[1], wants[1])
}

func TestAnyFloatsConcurrentDecodes(t *testing.T) {
	var wg sync.WaitGroup
	errs := make(chan string, slabTestWorkers)
	for w := range slabTestWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var kept [][]any
			var wants [][]float64
			for i := range slabTestDocs / slabTestWorkers {
				doc, want := slabTestDoc(w*slabTestDocs + i)
				var v any
				if err := alosjson.Unmarshal([]byte(doc), &v); err != nil {
					errs <- err.Error()
					return
				}
				kept, wants = append(kept, v.([]any)), append(wants, want)
			}
			runtime.GC()
			for i := range kept {
				for j, wv := range wants[i] {
					if kept[i][j] != any(wv) {
						errs <- "corrupted value"
						return
					}
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}

func slabMixedDoc(seed int) string {
	var b strings.Builder
	b.WriteString(`{"items":[`)
	for i := range slabTestDocValues {
		if i > 0 {
			b.WriteByte(',')
		}
		n := strconv.Itoa(seed*100 + i)
		b.WriteString(`{"s":"str-` + n + `","e":"esc\\` + n + `\n","a":[` + n + `,"x` + n + `",[],[true,null]],"f":` + n + `.5}`)
	}
	b.WriteString(`],"tail":["a","b",[1,[2,[3]]]]}`)
	return b.String()
}

func TestAnyStringsAndSlicesSurviveLaterDecodes(t *testing.T) {
	kept := make([]any, slabTestDocs)
	wants := make([]any, slabTestDocs)
	for i := range kept {
		doc := slabMixedDoc(i)
		if err := alosjson.Unmarshal([]byte(doc), &kept[i]); err != nil {
			t.Fatal(err)
		}
		if err := stdjson.Unmarshal([]byte(doc), &wants[i]); err != nil {
			t.Fatal(err)
		}
		if i%40 == 0 {
			runtime.GC()
		}
	}
	runtime.GC()
	for i := range kept {
		if !reflect.DeepEqual(kept[i], wants[i]) {
			t.Fatalf("doc %d changed after later decodes", i)
		}
	}
	items := kept[0].(map[string]any)["items"].([]any)
	items[0] = "replaced"
	if !reflect.DeepEqual(kept[1], wants[1]) {
		t.Fatal("mutating one result changed another")
	}
}

type slabNode struct {
	ID   int         `json:"id"`
	Name string      `json:"name"`
	Next *slabNode   `json:"next"`
	Kids []*slabNode `json:"kids"`
	P    *int        `json:"p"`
	S    *string     `json:"s"`
	B    *[3]byte    `json:"b"`
}

func slabNodeDoc(seed, depth int) string {
	n := strconv.Itoa(seed)
	if depth == 0 {
		return `{"id":` + n + `,"name":"leaf` + n + `","p":` + n + `,"s":"s` + n + `","b":[1,2,3]}`
	}
	return `{"id":` + n + `,"name":"n` + n + `","next":` + slabNodeDoc(seed+1, depth-1) +
		`,"kids":[` + slabNodeDoc(seed*3, depth-1) + `,null,` + slabNodeDoc(seed*3+1, depth-1) + `],"p":` + n + `}`
}

func TestPointersSurviveLaterDecodes(t *testing.T) {
	kept := make([]*slabNode, slabTestDocs/4)
	wants := make([]*slabNode, len(kept))
	for i := range kept {
		doc := slabNodeDoc(i, 3)
		if err := alosjson.Unmarshal([]byte(doc), &kept[i]); err != nil {
			t.Fatal(err)
		}
		if err := stdjson.Unmarshal([]byte(doc), &wants[i]); err != nil {
			t.Fatal(err)
		}
		if i%10 == 0 {
			runtime.GC()
		}
	}
	runtime.GC()
	for i := range kept {
		if !reflect.DeepEqual(kept[i], wants[i]) {
			t.Fatalf("doc %d changed after later decodes", i)
		}
	}
	kept[0].Next.Name = "mutated"
	*kept[0].P = -1
	kept[0].Next.Next.Next.B[0] = 9
	*kept[0].Kids[0].Kids[2].P = -2
	for i := 1; i < len(kept); i++ {
		if !reflect.DeepEqual(kept[i], wants[i]) {
			t.Fatalf("mutating doc 0 changed doc %d", i)
		}
	}
}

func TestPointersConcurrentDecodes(t *testing.T) {
	var wg sync.WaitGroup
	errs := make(chan string, slabTestWorkers)
	for w := range slabTestWorkers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var kept, wants []*slabNode
			for i := range 20 {
				doc := slabNodeDoc(w*100+i, 3)
				var got, want *slabNode
				if err := alosjson.Unmarshal([]byte(doc), &got); err != nil {
					errs <- err.Error()
					return
				}
				_ = stdjson.Unmarshal([]byte(doc), &want)
				kept, wants = append(kept, got), append(wants, want)
			}
			runtime.GC()
			for i := range kept {
				if !reflect.DeepEqual(kept[i], wants[i]) {
					errs <- "corrupted pointer graph"
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for e := range errs {
		t.Fatal(e)
	}
}

func TestChunkedSlicesAppendIndependently(t *testing.T) {
	var docs []*slabNode
	for i := range 64 {
		var n *slabNode
		if err := alosjson.Unmarshal([]byte(slabNodeDoc(i, 2)), &n); err != nil {
			t.Fatal(err)
		}
		docs = append(docs, n)
	}
	var wants []*slabNode
	for i := range docs {
		var w *slabNode
		_ = stdjson.Unmarshal([]byte(slabNodeDoc(i, 2)), &w)
		wants = append(wants, w)
	}
	extra := &slabNode{Name: "appended"}
	for i, d := range docs {
		before := len(d.Kids)
		if cap(d.Kids) == before {
			t.Fatalf("doc %d: expected spare capacity in decoded slice", i)
		}
		grown := append(d.Kids, extra)
		if &grown[0] != &d.Kids[0] {
			t.Fatalf("doc %d: in-capacity append reallocated", i)
		}
	}
	runtime.GC()
	for i := range docs {
		if !reflect.DeepEqual(docs[i], wants[i]) {
			t.Fatalf("appending to doc %d slices corrupted a decoded result", i)
		}
	}
}
