package json_test

import (
	"strconv"
	"strings"
	"testing"

	json "github.com/guno1928/alos-json"
)

type readmeUser struct {
	ID    int64    `json:"id"`
	Name  string   `json:"name"`
	Email string   `json:"email,omitempty"`
	Tags  []string `json:"tags"`
}

func TestReadmeQuickStart(t *testing.T) {
	data, err := json.Marshal(readmeUser{ID: 1, Name: "Ada", Tags: []string{"admin"}})
	if err != nil || string(data) != `{"id":1,"name":"Ada","tags":["admin"]}` {
		t.Fatalf("Marshal = %s, %v", data, err)
	}
	var u readmeUser
	if err := json.Unmarshal([]byte(`{"id":2,"name":"Grace","tags":[]}`), &u); err != nil || u.ID != 2 || u.Name != "Grace" {
		t.Fatalf("Unmarshal = %+v, %v", u, err)
	}
	if !json.Valid([]byte(`{"ok":true}`)) || json.Valid([]byte(`{"ok":}`)) {
		t.Fatal("Valid")
	}
}

func TestReadmeMarshalAppend(t *testing.T) {
	buf := make([]byte, 0, 1024)
	var err error
	for _, u := range []readmeUser{{ID: 1, Name: "a"}, {ID: 2, Name: "b"}} {
		buf, err = json.MarshalAppend(buf[:0], u)
		if err != nil {
			t.Fatal(err)
		}
	}
	if string(buf) != `{"id":2,"name":"b","tags":[]}` {
		t.Fatalf("MarshalAppend = %s", buf)
	}
}

func TestReadmeOptions(t *testing.T) {
	m := map[string]any{"b": "<x>", "a": []int(nil)}
	data, err := json.Marshal(m, json.Deterministic(true), json.EscapeHTML(true), json.FormatNilSliceAsNull(true), json.WithIndent("", "  "))
	if want := "{\n  \"a\": null,\n  \"b\": \"\\u003cx\\u003e\"\n}"; err != nil || string(data) != want {
		t.Fatalf("Marshal with options = %q, want %q (%v)", data, want, err)
	}
	var u readmeUser
	if err := json.Unmarshal([]byte(`{"NAME":"Ada"}`), &u, json.MatchCaseInsensitiveNames(true)); err != nil || u.Name != "Ada" {
		t.Fatalf("MatchCaseInsensitiveNames = %+v, %v", u, err)
	}
	if err := json.Unmarshal([]byte(`{"name":"Ada","extra":1}`), &u, json.RejectUnknownMembers(true)); err == nil {
		t.Fatal("RejectUnknownMembers accepted an unknown field")
	}
}

type readmeAddress struct {
	City string `json:"city"`
}

type readmeTagged struct {
	Code   int    `json:"code,string"`
	Note   string `json:"note,omitzero"`
	Count  int    `json:"count,omitempty"`
	Secret string `json:"-"`
	readmeAddress
}

func TestReadmeStructTags(t *testing.T) {
	data, err := json.Marshal(readmeTagged{Code: 7, Secret: "x", readmeAddress: readmeAddress{City: "Oslo"}})
	if want := `{"code":"7","city":"Oslo"}`; err != nil || string(data) != want {
		t.Fatalf("tags Marshal = %s, want %s (%v)", data, want, err)
	}
	var v readmeTagged
	if err := json.Unmarshal([]byte(`{"code":"9","city":"Rome","Secret":"y"}`), &v); err != nil || v.Code != 9 || v.City != "Rome" || v.Secret != "" {
		t.Fatalf("tags Unmarshal = %+v (%v)", v, err)
	}
}

func TestReadmeStdCompatible(t *testing.T) {
	type doc struct {
		Items []int          `json:"items"`
		Meta  map[string]int `json:"meta"`
		HTML  string         `json:"html"`
	}
	data, err := json.Marshal(doc{HTML: "<b>"},
		json.Deterministic(true),
		json.EscapeHTML(true),
		json.FormatNilSliceAsNull(true),
		json.FormatNilMapAsNull(true),
	)
	if want := `{"items":null,"meta":null,"html":"\u003cb\u003e"}`; err != nil || string(data) != want {
		t.Fatalf("std-compatible Marshal = %s, want %s (%v)", data, want, err)
	}
}

type readmeCelsius float64

func (c readmeCelsius) MarshalJSON() ([]byte, error) {
	return []byte(`{"celsius":` + strconv.FormatFloat(float64(c), 'f', -1, 64) + `}`), nil
}

func (c *readmeCelsius) UnmarshalJSON(b []byte) error {
	var v struct {
		Celsius float64 `json:"celsius"`
	}
	if err := json.Unmarshal(b, &v); err != nil {
		return err
	}
	*c = readmeCelsius(v.Celsius)
	return nil
}

func TestReadmeCustomMarshalers(t *testing.T) {
	type reading struct {
		Temp readmeCelsius `json:"temp"`
	}
	data, err := json.Marshal(reading{Temp: 21.5})
	if want := `{"temp":{"celsius":21.5}}`; err != nil || string(data) != want {
		t.Fatalf("Marshal custom = %s, want %s (%v)", data, want, err)
	}
	var r reading
	if err := json.Unmarshal([]byte(`{"temp":{"celsius":-4}}`), &r); err != nil || r.Temp != -4 {
		t.Fatalf("Unmarshal custom = %+v (%v)", r, err)
	}
}

type readmeZeroer struct{ v int }

func (z readmeZeroer) IsZero() bool { return z.v < 0 }

func TestReadmeTagSemantics(t *testing.T) {
	type omits struct {
		B    bool           `json:"b,omitempty"`
		F    float64        `json:"f,omitempty"`
		P    *int           `json:"p,omitempty"`
		I    any            `json:"i,omitempty"`
		S    string         `json:"s,omitempty"`
		Sl   []int          `json:"sl,omitempty"`
		M    map[string]int `json:"m,omitempty"`
		Z    readmeZeroer   `json:"z,omitzero"`
		Q    bool           `json:"q,string"`
		Keep map[string]int `json:"keep"`
	}
	data, err := json.Marshal(omits{Sl: []int{}, M: map[string]int{}, Z: readmeZeroer{v: -1}, Q: true})
	if want := `{"q":"true","keep":{}}`; err != nil || string(data) != want {
		t.Fatalf("omit semantics = %s, want %s (%v)", data, want, err)
	}
	var back omits
	if err := json.Unmarshal([]byte(`{"q":"true"}`), &back); err != nil || !back.Q {
		t.Fatalf("string bool decode = %+v (%v)", back, err)
	}
}

func TestReadmeNestingLimit(t *testing.T) {
	const limit = 10_000
	var v any
	if err := json.Unmarshal([]byte(strings.Repeat("[", limit)+strings.Repeat("]", limit)), &v); err != nil {
		t.Fatalf("depth %d rejected: %v", limit, err)
	}
	if err := json.Unmarshal([]byte(strings.Repeat("[", limit+1)+strings.Repeat("]", limit+1)), &v); err == nil {
		t.Fatalf("depth %d accepted", limit+1)
	}
}

func TestQuotedBoolsBothDecodePaths(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want bool
		ok   bool
	}{
		{`"true"`, true, true}, {`"false"`, false, true}, {`true`, true, true}, {`false`, false, true},
		{`"TRUE"`, false, false}, {`"true "`, false, false}, {`"tru"`, false, false}, {`"true`, false, false}, {`""`, false, false}, {`1`, false, false},
	} {
		var fast bool
		errFast := json.Unmarshal([]byte(tc.in), &fast)
		var slow bool
		errSlow := json.UnmarshalRead(strings.NewReader(tc.in), &slow)
		for _, r := range []struct {
			name string
			got  bool
			err  error
		}{{"Unmarshal", fast, errFast}, {"UnmarshalRead", slow, errSlow}} {
			if (r.err == nil) != tc.ok || tc.ok && r.got != tc.want {
				t.Errorf("%s(%s) = %v, %v; want %v, ok=%v", r.name, tc.in, r.got, r.err, tc.want, tc.ok)
			}
		}
	}
	type flags struct {
		On bool `json:"on,string"`
	}
	data, err := json.Marshal(flags{On: true})
	var back flags
	if err != nil || json.Unmarshal(data, &back) != nil || !back.On {
		t.Fatalf("string-tagged bool round trip failed: %s -> %+v (%v)", data, back, err)
	}
}

type readmeCelsiusText float64

func TestReadmeMarshalFunc(t *testing.T) {
	asText := json.MarshalFunc(func(c readmeCelsiusText) ([]byte, error) {
		return []byte(strconv.Quote(strconv.FormatFloat(float64(c), 'f', 1, 64) + "C")), nil
	})
	data, err := json.Marshal(map[string]readmeCelsiusText{"office": 21.5}, json.WithMarshalers(asText))
	if want := `{"office":"21.5C"}`; err != nil || string(data) != want {
		t.Fatalf("MarshalFunc example = %s, want %s (%v)", data, want, err)
	}
}

func TestReadmeOptionsAndInline(t *testing.T) {
	type extra struct {
		ID   int            `json:"id"`
		Rest map[string]int `json:",inline"`
	}
	var v extra
	if err := json.Unmarshal([]byte(`{"id":1,"a":2}`), &v, json.RejectUnknownMembers(true)); err != nil || v.Rest["a"] != 2 {
		t.Fatalf("inline map decode = %+v (%v)", v, err)
	}
	if _, err := json.Marshal(extra{ID: 1, Rest: map[string]int{"id": 3}}); err == nil {
		t.Fatal("inline map key clashing with a field did not fail")
	}
	data, err := json.Marshal(struct{ N int }{42}, json.StringifyNumbers(true))
	if want := `{"N":"42"}`; err != nil || string(data) != want {
		t.Fatalf("StringifyNumbers = %s, want %s (%v)", data, want, err)
	}
	data, err = json.Marshal(struct{ A, B int }{B: 1}, json.OmitZeroStructFields(true))
	if want := `{"B":1}`; err != nil || string(data) != want {
		t.Fatalf("OmitZeroStructFields = %s, want %s (%v)", data, want, err)
	}
}
