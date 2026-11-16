package json_test

import (
	"testing"

	alosjson "github.com/guno1928/alos-json"
)

type allocDocType struct {
	Name  string          `json:"name"`
	Age   int             `json:"age"`
	Email string          `json:"email"`
	Score float64         `json:"score"`
	Tags  []string        `json:"tags"`
	IDs   []int64         `json:"ids"`
	Vals  []float64       `json:"vals"`
	Kids  []*allocDocType `json:"kids"`
}

const (
	allocRuns     = 200
	poolRefillPad = 1
)

var allocDoc = []byte(`{"name":"alice","age":30,"email":"a@example.com","score":98.5,"tags":["x","y","z"],"ids":[1,2,3,40000000000],"vals":[1.5,2.25,-3e10],"kids":[{"name":"bob","age":3,"kids":[]},{"name":"c\nd","age":4}]}`)

func TestAllocationBudgets(t *testing.T) {
	if instrumentedBuild {
		t.Skip("allocation counts are not meaningful under race, asan or msan instrumentation")
	}
	var reused allocDocType
	if err := alosjson.Unmarshal(allocDoc, &reused); err != nil {
		t.Fatal(err)
	}
	var generic any
	if err := alosjson.Unmarshal(allocDoc, &generic); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 0, 4096)
	cases := []struct {
		name   string
		budget float64
		fn     func()
	}{
		{"Unmarshal typed fresh", 4 + poolRefillPad, func() { var x allocDocType; _ = alosjson.Unmarshal(allocDoc, &x) }},
		{"Unmarshal typed reused", 0 + poolRefillPad, func() { _ = alosjson.Unmarshal(allocDoc, &reused) }},
		{"Unmarshal any", 11 + poolRefillPad, func() { var x any; _ = alosjson.Unmarshal(allocDoc, &x) }},
		{"Marshal typed", 1, func() { _, _ = alosjson.Marshal(&reused) }},
		{"Marshal any", 1, func() { _, _ = alosjson.Marshal(generic) }},
		{"MarshalAppend typed", 0, func() { _, _ = alosjson.MarshalAppend(buf[:0], &reused) }},
		{"Valid", 0, func() { _ = alosjson.Valid(allocDoc) }},
	}
	for _, c := range cases {
		if got := testing.AllocsPerRun(allocRuns, c.fn); got > c.budget {
			t.Errorf("%s: %v allocs/op, budget %v", c.name, got, c.budget)
		}
	}
}
