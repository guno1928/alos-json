package json_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/bytedance/sonic"

	alosjson "github.com/guno1928/alos-json"
	"github.com/guno1928/alos-json/jsontext"
)

type SmallStruct struct {
	Name  string  `json:"name"`
	Age   int     `json:"age"`
	Email string  `json:"email"`
	Score float64 `json:"score"`
}

type MediumStruct struct {
	ID      uint64   `json:"id"`
	Name    string   `json:"name"`
	Email   string   `json:"email"`
	Age     int      `json:"age"`
	Score   float64  `json:"score"`
	Active  bool     `json:"active"`
	Tags    []string `json:"tags"`
	Address Address  `json:"address"`
}

type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
	State  string `json:"state"`
	Zip    string `json:"zip"`
}

type LargeStruct struct {
	Users []MediumStruct `json:"users"`
	Total int            `json:"total"`
	Page  int            `json:"page"`
}

var smallData = SmallStruct{
	Name:  "Alice",
	Age:   30,
	Email: "alice@example.com",
	Score: 98.5,
}

var mediumData = MediumStruct{
	ID:     12345,
	Name:   "Bob Smith",
	Email:  "bob@example.com",
	Age:    42,
	Score:  87.3,
	Active: true,
	Tags:   []string{"admin", "user", "developer"},
	Address: Address{
		Street: "123 Main St",
		City:   "Springfield",
		State:  "IL",
		Zip:    "62701",
	},
}

func makeLargeData() LargeStruct {
	users := make([]MediumStruct, 100)
	for i := range users {
		users[i] = MediumStruct{
			ID:     uint64(i + 1),
			Name:   fmt.Sprintf("User %d", i),
			Email:  fmt.Sprintf("user%d@example.com", i),
			Age:    20 + (i % 50),
			Score:  float64(50 + (i % 50)),
			Active: i%2 == 0,
			Tags:   []string{"tag1", "tag2"},
			Address: Address{
				Street: fmt.Sprintf("%d Elm St", i),
				City:   "Springfield",
				State:  "IL",
				Zip:    "62701",
			},
		}
	}
	return LargeStruct{Users: users, Total: 100, Page: 1}
}

var largeData = makeLargeData()

var (
	smallJSON  []byte
	mediumJSON []byte
	largeJSON  []byte
)

func init() {
	smallJSON, _ = json.Marshal(smallData)
	mediumJSON, _ = json.Marshal(mediumData)
	largeJSON, _ = json.Marshal(largeData)
}

func BenchmarkMarshal_Small_StdLib(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		json.Marshal(smallData)
	}
}

func BenchmarkMarshal_Small_Sonic(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sonic.Marshal(smallData)
	}
}

func BenchmarkMarshal_Small_Alos(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		alosjson.Marshal(smallData)
	}
}

func BenchmarkMarshal_Medium_StdLib(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		json.Marshal(mediumData)
	}
}

func BenchmarkMarshal_Medium_Sonic(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sonic.Marshal(mediumData)
	}
}

func BenchmarkMarshal_Medium_Alos(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		alosjson.Marshal(mediumData)
	}
}

func BenchmarkMarshal_Large_StdLib(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		json.Marshal(largeData)
	}
}

func BenchmarkMarshal_Large_Sonic(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		sonic.Marshal(largeData)
	}
}

func BenchmarkMarshal_Large_Alos(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		alosjson.Marshal(largeData)
	}
}

func BenchmarkUnmarshal_Small_StdLib(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v SmallStruct
		json.Unmarshal(smallJSON, &v)
	}
}

func BenchmarkUnmarshal_Small_Sonic(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v SmallStruct
		sonic.Unmarshal(smallJSON, &v)
	}
}

func BenchmarkUnmarshal_Small_Alos(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v SmallStruct
		alosjson.Unmarshal(smallJSON, &v)
	}
}

func BenchmarkUnmarshal_Medium_StdLib(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v MediumStruct
		json.Unmarshal(mediumJSON, &v)
	}
}

func BenchmarkUnmarshal_Medium_Sonic(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v MediumStruct
		sonic.Unmarshal(mediumJSON, &v)
	}
}

func BenchmarkUnmarshal_Medium_Alos(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v MediumStruct
		alosjson.Unmarshal(mediumJSON, &v)
	}
}

func BenchmarkUnmarshal_Large_StdLib(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v LargeStruct
		json.Unmarshal(largeJSON, &v)
	}
}

func BenchmarkUnmarshal_Large_Sonic(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v LargeStruct
		sonic.Unmarshal(largeJSON, &v)
	}
}

func BenchmarkUnmarshal_Large_Alos(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v LargeStruct
		alosjson.Unmarshal(largeJSON, &v)
	}
}

func BenchmarkStreamingDecode_StdLib(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		dec := json.NewDecoder(bytes.NewReader(largeJSON))
		for dec.More() {
			var v json.RawMessage
			dec.Decode(&v)
		}
	}
}

func BenchmarkStreamingDecode_Alos(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		dec := jsontext.NewDecoderFromBytes(largeJSON)
		for {
			_, err := dec.ReadToken()
			if err != nil {
				break
			}
		}
		jsontext.PutDecoder(dec)
	}
}

func BenchmarkTokenScan_Alos(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSON)))
	for b.Loop() {
		dec := jsontext.NewDecoderFromBytes(largeJSON)
		for {
			_, err := dec.ReadToken()
			if err != nil {
				break
			}
		}
		jsontext.PutDecoder(dec)
	}
}

func BenchmarkTokenEncode_Alos(b *testing.B) {
	b.ReportAllocs()
	var buf []byte
	for b.Loop() {
		buf = buf[:0]
		enc := jsontext.NewEncoderToBytes(&buf)
		enc.WriteToken(jsontext.TBeginObject)
		enc.WriteString("name")
		enc.WriteString("Alice")
		enc.WriteString("age")
		enc.WriteInt(30)
		enc.WriteString("scores")
		enc.WriteToken(jsontext.TBeginArray)
		for i := 0; i < 10; i++ {
			enc.WriteFloat(float64(i) * 1.5)
		}
		enc.WriteToken(jsontext.TEndArray)
		enc.WriteToken(jsontext.TEndObject)
	}
}

func BenchmarkMarshalMap_StdLib(b *testing.B) {
	m := map[string]any{
		"name":   "Alice",
		"age":    30,
		"active": true,
		"scores": []any{1.0, 2.0, 3.0, 4.0, 5.0},
	}
	b.ReportAllocs()
	for b.Loop() {
		json.Marshal(m)
	}
}

func BenchmarkMarshalMap_Sonic(b *testing.B) {
	m := map[string]any{
		"name":   "Alice",
		"age":    30,
		"active": true,
		"scores": []any{1.0, 2.0, 3.0, 4.0, 5.0},
	}
	b.ReportAllocs()
	for b.Loop() {
		sonic.Marshal(m)
	}
}

func BenchmarkMarshalMap_Alos(b *testing.B) {
	m := map[string]any{
		"name":   "Alice",
		"age":    30,
		"active": true,
		"scores": []any{1.0, 2.0, 3.0, 4.0, 5.0},
	}
	b.ReportAllocs()
	for b.Loop() {
		alosjson.Marshal(m)
	}
}

func BenchmarkUnmarshalMap_StdLib(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v map[string]any
		json.Unmarshal(mediumJSON, &v)
	}
}

func BenchmarkUnmarshalMap_Sonic(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v map[string]any
		sonic.Unmarshal(mediumJSON, &v)
	}
}

func BenchmarkUnmarshalMap_Alos(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		var v map[string]any
		alosjson.Unmarshal(mediumJSON, &v)
	}
}

func BenchmarkValid_StdLib(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSON)))
	for b.Loop() {
		json.Valid(largeJSON)
	}
}

func BenchmarkValid_Sonic(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSON)))
	for b.Loop() {
		sonic.Valid(largeJSON)
	}
}

func BenchmarkValid_Alos(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(largeJSON)))
	for b.Loop() {
		alosjson.Valid(largeJSON)
	}
}

var indentedJSON []byte

func init() {
	var buf bytes.Buffer
	json.Indent(&buf, largeJSON, "", "  ")
	indentedJSON = buf.Bytes()
}

func BenchmarkCompact_StdLib(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(indentedJSON)))
	for b.Loop() {
		var buf bytes.Buffer
		json.Compact(&buf, indentedJSON)
	}
}

func BenchmarkCompact_Alos(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(indentedJSON)))
	for b.Loop() {
		var dst []byte
		alosjson.Compact(&dst, indentedJSON)
	}
}

func BenchmarkQuoteString_Alos(b *testing.B) {
	s := `Hello "world" with\special <chars> & more`
	b.ReportAllocs()
	var buf [128]byte
	for b.Loop() {
		jsontext.AppendQuote(buf[:0], s)
	}
}

func BenchmarkMarshalAppend_Alos(b *testing.B) {
	b.ReportAllocs()
	buf := make([]byte, 0, 4096)
	for b.Loop() {
		buf = buf[:0]
		alosjson.MarshalAppend(buf, smallData)
	}
}

func BenchmarkDeepNested_Alos(b *testing.B) {
	depth := 100
	var sb strings.Builder
	for i := 0; i < depth; i++ {
		sb.WriteString(`{"a":`)
	}
	sb.WriteString(`1`)
	for i := 0; i < depth; i++ {
		sb.WriteByte('}')
	}
	data := []byte(sb.String())
	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	for b.Loop() {
		dec := jsontext.NewDecoderFromBytes(data)
		for {
			_, err := dec.ReadToken()
			if err != nil {
				break
			}
		}
		jsontext.PutDecoder(dec)
	}
}

func TestMarshalUnmarshalSmall(t *testing.T) {
	b, err := alosjson.Marshal(smallData)
	if err != nil {
		t.Fatal(err)
	}
	var out SmallStruct
	if err := alosjson.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != smallData.Name || out.Age != smallData.Age || out.Email != smallData.Email {
		t.Fatalf("roundtrip mismatch: got %+v, want %+v", out, smallData)
	}
}

func TestMarshalUnmarshalMedium(t *testing.T) {
	b, err := alosjson.Marshal(mediumData)
	if err != nil {
		t.Fatal(err)
	}
	var out MediumStruct
	if err := alosjson.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != mediumData.Name || out.ID != mediumData.ID || out.Active != mediumData.Active {
		t.Fatalf("roundtrip mismatch: got %+v", out)
	}
	if out.Address.City != mediumData.Address.City {
		t.Fatalf("nested mismatch: got %+v", out.Address)
	}
	if len(out.Tags) != len(mediumData.Tags) {
		t.Fatalf("tags mismatch: got %v, want %v", out.Tags, mediumData.Tags)
	}
}

func TestMarshalUnmarshalLarge(t *testing.T) {
	b, err := alosjson.Marshal(largeData)
	if err != nil {
		t.Fatal(err)
	}
	var out LargeStruct
	if err := alosjson.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != largeData.Total || len(out.Users) != len(largeData.Users) {
		t.Fatalf("large roundtrip mismatch: total=%d users=%d", out.Total, len(out.Users))
	}
}

func TestUnmarshalMap(t *testing.T) {
	data := []byte(`{"name":"Alice","age":30,"active":true}`)
	var out map[string]any
	if err := alosjson.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out["name"] != "Alice" {
		t.Fatalf("expected Alice, got %v", out["name"])
	}
	if out["age"] != float64(30) {
		t.Fatalf("expected 30, got %v", out["age"])
	}
}

func TestOmitEmpty(t *testing.T) {
	type S struct {
		Name  string `json:"name"`
		Value string `json:"value,omitempty"`
	}
	b, err := alosjson.Marshal(S{Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("value")) {
		t.Fatalf("expected value omitted, got: %s", b)
	}
}

func TestOmitZero(t *testing.T) {
	type S struct {
		Name  string `json:"name"`
		Count int    `json:"count,omitzero"`
	}
	b, err := alosjson.Marshal(S{Name: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("count")) {
		t.Fatalf("expected count omitted, got: %s", b)
	}
}

func TestNullHandling(t *testing.T) {
	type S struct {
		Name *string `json:"name"`
	}
	data := []byte(`{"name":null}`)
	var out S
	if err := alosjson.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	if out.Name != nil {
		t.Fatalf("expected nil, got %v", out.Name)
	}
}

func TestValid(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{`{}`, true},
		{`{"a":1}`, true},
		{`[1,2,3]`, true},
		{`"hello"`, true},
		{`42`, true},
		{`null`, true},
		{`true`, true},
		{`false`, true},
		{`{invalid}`, false},
		{``, false},
		{`{"a":}`, false},
	}
	for _, tt := range tests {
		if got := alosjson.Valid([]byte(tt.in)); got != tt.want {
			t.Errorf("Valid(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}

func TestStreamingDecoder(t *testing.T) {
	data := []byte(`{"name":"Alice","age":30}`)
	dec := jsontext.NewDecoderFromBytes(data)
	expectations := []jsontext.Kind{
		jsontext.BeginObject,
		jsontext.String,
		jsontext.String,
		jsontext.String,
		jsontext.Number,
		jsontext.EndObject,
	}
	for _, expect := range expectations {
		tok, err := dec.ReadToken()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if tok.Kind() != expect {
			t.Fatalf("expected kind %v, got %v", expect, tok.Kind())
		}
	}
	jsontext.PutDecoder(dec)
}

func TestEscapeHTML(t *testing.T) {
	type S struct {
		HTML string `json:"html"`
	}
	b, err := alosjson.Marshal(S{HTML: "<script>alert('xss')</script>"}, alosjson.EscapeHTML(true))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("<script>")) {
		t.Fatalf("expected HTML escaped, got: %s", b)
	}
}

func TestRejectUnknownMembers(t *testing.T) {
	type S struct {
		Name string `json:"name"`
	}
	data := []byte(`{"name":"Alice","extra":"field"}`)
	var out S
	err := alosjson.Unmarshal(data, &out, alosjson.RejectUnknownMembers(true))
	if err == nil {
		t.Fatal("expected error for unknown field")
	}
}

func TestCaseInsensitive(t *testing.T) {
	type S struct {
		Name string `json:"name"`
	}
	data := []byte(`{"NAME":"Alice"}`)
	var out S
	err := alosjson.Unmarshal(data, &out, alosjson.MatchCaseInsensitiveNames(true))
	if err != nil {
		t.Fatal(err)
	}
	if out.Name != "Alice" {
		t.Fatalf("expected Alice, got %v", out.Name)
	}
}

func TestNilSlice(t *testing.T) {
	type S struct {
		Items []string `json:"items"`
	}
	b, _ := alosjson.Marshal(S{})
	if !bytes.Contains(b, []byte(`"items":[]`)) {
		t.Fatalf("expected empty array, got: %s", b)
	}
	b, _ = alosjson.Marshal(S{}, alosjson.FormatNilSliceAsNull(true))
	if !bytes.Contains(b, []byte(`"items":null`)) {
		t.Fatalf("expected null, got: %s", b)
	}
}

func TestNilMap(t *testing.T) {
	type S struct {
		Data map[string]string `json:"data"`
	}
	b, _ := alosjson.Marshal(S{})
	if !bytes.Contains(b, []byte(`"data":{}`)) {
		t.Fatalf("expected empty object, got: %s", b)
	}
	b, _ = alosjson.Marshal(S{}, alosjson.FormatNilMapAsNull(true))
	if !bytes.Contains(b, []byte(`"data":null`)) {
		t.Fatalf("expected null, got: %s", b)
	}
}

func TestStringTag(t *testing.T) {
	type S struct {
		Count int `json:"count,string"`
	}
	b, err := alosjson.Marshal(S{Count: 42})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte(`"count":"42"`)) {
		t.Fatalf("expected stringified number, got: %s", b)
	}
}

func TestBase64Bytes(t *testing.T) {
	type S struct {
		Data []byte `json:"data"`
	}
	original := S{Data: []byte("hello world")}
	b, err := alosjson.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	var out S
	if err := alosjson.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if string(out.Data) != "hello world" {
		t.Fatalf("expected 'hello world', got '%s'", out.Data)
	}
}

func TestNestedPointer(t *testing.T) {
	type Inner struct {
		X int `json:"x"`
	}
	type Outer struct {
		Inner *Inner `json:"inner"`
	}
	b, _ := alosjson.Marshal(Outer{Inner: &Inner{X: 42}})
	var out Outer
	if err := alosjson.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if out.Inner == nil || out.Inner.X != 42 {
		t.Fatalf("nested pointer roundtrip failed: %+v", out)
	}
}
