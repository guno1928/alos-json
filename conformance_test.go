package json_test

import (
	stdjson "encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	alosjson "github.com/guno1928/alos-json"
	"github.com/guno1928/alos-json/jsontext"
)

const jsonTestSuiteEnv = "JSONTESTSUITE_DIR"

func unmarshalNoPanic(t *testing.T, name string, data []byte, v any) (err error) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("%s: Unmarshal panicked: %v", name, r)
		}
	}()
	return alosjson.Unmarshal(data, v)
}

func TestJSONTestSuite(t *testing.T) {
	dir := os.Getenv(jsonTestSuiteEnv)
	if dir == "" {
		t.Skip(jsonTestSuiteEnv + " not set")
	}
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no test files in %s: %v", dir, err)
	}
	counts := map[string]int{}
	for _, path := range files {
		name := filepath.Base(path)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		kind := name[:2]
		counts[kind]++
		valid := alosjson.Valid(data)
		if jsontext.Value(data).IsValid() != valid {
			t.Errorf("%s: IsValid disagrees with Valid", name)
		}
		var got any
		decodeErr := unmarshalNoPanic(t, name, data, &got)
		var raw []any
		unmarshalNoPanic(t, name, data, &raw)
		var typed struct {
			A any `json:"a"`
		}
		unmarshalNoPanic(t, name, data, &typed)
		switch kind {
		case "y_":
			if !valid {
				t.Errorf("%s: Valid = false for a document that must be accepted", name)
			}
			if decodeErr != nil {
				t.Errorf("%s: Unmarshal error %v for a document that must be accepted", name, decodeErr)
				continue
			}
			var want any
			if err := stdjson.Unmarshal(data, &want); err == nil && strings.ToValidUTF8(string(data), "") == string(data) && !reflect.DeepEqual(got, want) {
				t.Errorf("%s: Unmarshal = %#v, encoding/json = %#v", name, got, want)
			}
			out, err := alosjson.Marshal(got)
			if err != nil {
				t.Errorf("%s: Marshal of decoded value failed: %v", name, err)
				continue
			}
			if !alosjson.Valid(out) || !stdjson.Valid(out) {
				t.Errorf("%s: Marshal produced invalid JSON %q", name, out)
			}
			var again any
			if err := alosjson.Unmarshal(out, &again); err != nil || !reflect.DeepEqual(again, got) {
				t.Errorf("%s: round trip changed the value: %#v -> %q -> %#v (%v)", name, got, out, again, err)
			}
		case "n_":
			if valid {
				t.Errorf("%s: Valid = true for a document that must be rejected: %q", name, data)
			}
			if decodeErr == nil {
				t.Errorf("%s: Unmarshal accepted a document that must be rejected: %q", name, data)
			}
		case "i_":
			if want := stdjson.Valid(data); valid != want {
				t.Logf("%s: implementation-defined: Valid = %v, encoding/json = %v", name, valid, want)
			}
		}
	}
	t.Logf("checked %d accept, %d reject, %d implementation-defined cases", counts["y_"], counts["n_"], counts["i_"])
}
