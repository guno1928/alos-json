package json

import (
	"bytes"
	stdjson "encoding/json"
	"strings"
	"testing"
)

type handoffDoc struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

type handoffPair struct {
	First  string   `json:"first"`
	Second string   `json:"second"`
	Tail   []string `json:"tail"`
}

func handoffBody(fill byte, n int) string {
	return strings.Repeat(string(fill), n)
}

func stdMarshalNoHTML(t *testing.T, v any) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := stdjson.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		t.Fatal(err)
	}
	return bytes.TrimSuffix(buf.Bytes(), []byte("\n"))
}

func TestMarshalLargeStringResultsDoNotAlias(t *testing.T) {
	sizes := []int{handoffStringLen - 1, handoffStringLen, handoffStringLen + 1, 1 << 20}
	for _, n := range sizes {
		first := handoffDoc{Title: "one", Body: handoffBody('a', n)}
		second := handoffDoc{Title: "two", Body: handoffBody('b', n) + "\n\"" + handoffBody('c', n/3)}
		want1, want2 := stdMarshalNoHTML(t, first), stdMarshalNoHTML(t, second)
		got1, err := Marshal(first)
		if err != nil {
			t.Fatal(err)
		}
		got2, err := Marshal(second)
		if err != nil {
			t.Fatal(err)
		}
		got3, err := Marshal(&first)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got1, want1) || !bytes.Equal(got2, want2) || !bytes.Equal(got3, want1) {
			t.Fatalf("size %d: large string results differ from encoding/json or were overwritten", n)
		}
		got1[0] = 'X'
		if again, _ := Marshal(first); !bytes.Equal(again, want1) {
			t.Fatalf("size %d: mutating a returned buffer changed a later result", n)
		}
	}
}

func TestMarshalLargeStringFollowedByGrowth(t *testing.T) {
	tail := make([]string, 4096)
	for i := range tail {
		tail[i] = strings.Repeat("t", i%40)
	}
	cases := []handoffPair{
		{First: handoffBody('x', handoffStringLen), Second: "small", Tail: []string{}},
		{First: handoffBody('x', handoffStringLen), Second: handoffBody('y', 3*handoffStringLen), Tail: []string{}},
		{First: "small", Second: handoffBody('y', 2*handoffStringLen), Tail: tail},
		{First: handoffBody('x', 4*handoffStringLen), Tail: tail},
	}
	for i, c := range cases {
		want := stdMarshalNoHTML(t, c)
		for round := range 3 {
			got, err := Marshal(c)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("case %d round %d: output differs from encoding/json", i, round)
			}
		}
	}
}

func TestMarshalAppendLargeString(t *testing.T) {
	doc := handoffDoc{Title: "t", Body: handoffBody('q', 2*handoffStringLen)}
	body := stdMarshalNoHTML(t, doc)
	if got, err := MarshalAppend(nil, doc); err != nil || !bytes.Equal(got, body) {
		t.Fatalf("MarshalAppend(nil) with a large string is wrong: %v", err)
	}
	want := append([]byte("prefix"), body...)
	for _, dst := range [][]byte{[]byte("prefix"), append(make([]byte, 0, 3*handoffStringLen), "prefix"...)} {
		got, err := MarshalAppend(dst, doc)
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("MarshalAppend with a large string is wrong: %v", err)
		}
	}
}

func TestMarshalLargeStringIndentedAndEscaped(t *testing.T) {
	doc := handoffDoc{Title: "t", Body: handoffBody('<', handoffStringLen) + "\u2028"}
	got, err := Marshal(doc, EscapeHTML(true))
	if err != nil {
		t.Fatal(err)
	}
	want, err := stdjson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("EscapeHTML large string differs from encoding/json")
	}
	indented, err := Marshal(doc, WithIndent("", "  "))
	if err != nil {
		t.Fatal(err)
	}
	if !Valid(indented) || !bytes.Contains(indented, []byte(handoffBody('<', 64))) {
		t.Fatal("indented large string output is wrong")
	}
}
