package json

import (
	stdjson "encoding/json"
	"reflect"
	"testing"
)

type memberSpaceTarget struct {
	Name  string  `json:"name"`
	Age   int     `json:"age"`
	Score float64 `json:"score"`
	Tags  []int   `json:"tags"`
}

func TestStructMemberWhitespaceVariants(t *testing.T) {
	inputs := []string{
		`{"name":"a","age":1,"score":2.5,"tags":[1,2]}`,
		`{ "name" : "a" , "age" : 1 , "score" : 2.5 , "tags" : [ 1 , 2 ] }`,
		"{\n\t\"name\":\t\"a\",\n\t\"age\":\n1,\r\n\"score\":2.5,\"tags\":[1,2]\n}",
		`{"name": "a","age": 1}`,
		`{"age":1,"name":"a"}`,
		`{"name":"a","unknown":{"x":[1,2]},"age":1}`,
		`{"name":`,
		`{"name": `,
		`{"name"`,
		`{"name" "a"}`,
		`{"name":"a",}`,
		`{"name":"a" "age":1}`,
		`{ }`,
		`{"name":"a"`,
	}
	for _, in := range inputs {
		var got, want memberSpaceTarget
		gotErr := Unmarshal([]byte(in), &got)
		wantErr := stdjson.Unmarshal([]byte(in), &want)
		if (gotErr == nil) != (wantErr == nil) {
			t.Errorf("%q: error mismatch: got %v, std %v", in, gotErr, wantErr)
			continue
		}
		if gotErr == nil && !reflect.DeepEqual(got, want) {
			t.Errorf("%q: got %+v, std %+v", in, got, want)
		}
	}
}
