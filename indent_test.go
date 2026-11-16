package json_test

import (
	"bytes"
	stdjson "encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	alosjson "github.com/guno1928/alos-json"
)

type indentRecord struct {
	Name     string            `json:"name"`
	Tags     []string          `json:"tags"`
	Empty    []int             `json:"empty"`
	Meta     map[string]any    `json:"meta"`
	Nested   *indentRecord     `json:"nested,omitempty"`
	Labels   map[string]string `json:"labels"`
	Escaped  string            `json:"escaped"`
	Score    float64           `json:"score"`
	Children []indentRecord    `json:"children"`
}

var stdCompatibleOptions = []alosjson.Option{
	alosjson.Deterministic(true),
	alosjson.EscapeHTML(true),
	alosjson.FormatNilSliceAsNull(true),
	alosjson.FormatNilMapAsNull(true),
}

func checkIndentAgainstStd(t *testing.T, v any, prefix, indent string) {
	t.Helper()
	compact, err := alosjson.Marshal(v, stdCompatibleOptions...)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var want bytes.Buffer
	if err := stdjson.Indent(&want, compact, prefix, indent); err != nil {
		t.Fatalf("encoding/json.Indent rejected %q: %v", compact, err)
	}
	opts := append(append([]alosjson.Option(nil), stdCompatibleOptions...), alosjson.WithIndent(prefix, indent))
	got, err := alosjson.Marshal(v, opts...)
	if err != nil || !bytes.Equal(got, want.Bytes()) {
		t.Fatalf("Marshal(%#v, WithIndent(%q, %q)) =\n%s\n(err %v), encoding/json.Indent =\n%s", v, prefix, indent, got, err, want.Bytes())
	}
	appended, err := alosjson.MarshalAppend([]byte("existing-bytes:"), v, opts...)
	if err != nil || !bytes.Equal(appended, append([]byte("existing-bytes:"), want.Bytes()...)) {
		t.Fatalf("MarshalAppend with WithIndent = %q (err %v), want prefix + %q", appended, err, want.Bytes())
	}
}

func TestMarshalWithIndentMatchesStdIndent(t *testing.T) {
	rec := indentRecord{
		Name:    "root",
		Tags:    []string{"a", "b"},
		Empty:   []int{},
		Meta:    map[string]any{"z": 1.5, "a": []any{}, "m": map[string]any{}, "q": `quote " and \ slash`, "html": "<b>&</b>"},
		Labels:  map[string]string{},
		Escaped: "line\nbreak\t\"quoted\" \xe2\x80\xa8 é",
		Score:   -0.000125,
		Nested:  &indentRecord{Name: "child", Children: []indentRecord{{Name: "leaf"}}},
	}
	values := []any{
		rec, []any{}, map[string]any{}, []any{[]any{}, map[string]any{}, []any{[]any{1.0}}},
		"plain", 42, nil, true, []int{1, 2, 3}, map[string]int{"b": 2, "a": 1},
		map[string]any{"{[,:]}": "{[,:]}", `"\"`: []any{`"\\"`}},
	}
	for _, v := range values {
		for _, layout := range [][2]string{{"", "  "}, {"", "\t"}, {">>", "  "}, {"#", ""}} {
			checkIndentAgainstStd(t, v, layout[0], layout[1])
		}
	}
}

func TestMarshalWithIndentRandomDocs(t *testing.T) {
	g := &validDocGen{x: 0x2545F4914F6CDD1D}
	checked := 0
	for range 3_000 {
		var sb strings.Builder
		g.value(&sb, 0)
		doc := sb.String()
		if !utf8.ValidString(doc) {
			continue
		}
		var v any
		if err := stdjson.Unmarshal([]byte(doc), &v); err != nil {
			t.Fatalf("generator produced invalid JSON %q: %v", doc, err)
		}
		checkIndentAgainstStd(t, v, "", "  ")
		checked++
	}
	if checked < 1_500 {
		t.Fatalf("only %d documents checked", checked)
	}
}

func TestMarshalWithEmptyIndentIsCompact(t *testing.T) {
	v := map[string]any{"a": []any{1.0, map[string]any{"b": "c"}}}
	compact, err := alosjson.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	got, err := alosjson.Marshal(v, alosjson.WithIndent("", ""))
	if err != nil || !bytes.Equal(got, compact) {
		t.Fatalf("WithIndent with empty prefix and indent = %q (err %v), want compact %q", got, err, compact)
	}
}
