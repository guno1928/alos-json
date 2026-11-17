package json_test

import (
	"bytes"
	stdjson "encoding/json"
	"testing"
	"time"

	alosjson "github.com/guno1928/alos-json"
)

type timeHolder struct {
	At      time.Time  `json:"at"`
	Ptr     *time.Time `json:"ptr"`
	OmitPtr *time.Time `json:"omit_ptr,omitempty"`
	List    []time.Time
	Map     map[string]time.Time
	Any     any
}

func timeEdgeCases() []time.Time {
	zones := []*time.Location{
		time.UTC,
		time.FixedZone("plus", 5*3600+30*60),
		time.FixedZone("minus", -(9*3600 + 45*60)),
		time.FixedZone("max", 23*3600+59*60),
		time.FixedZone("hour24", 24*3600),
		time.FixedZone("seconds", 3600+30),
		time.FixedZone("negseconds", -(2*3600 + 7)),
	}
	instants := []time.Time{
		{},
		time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC),
		time.Date(2025, 3, 4, 5, 6, 7, 123456789, time.UTC),
		time.Date(2025, 3, 4, 5, 6, 7, 100000000, time.UTC),
		time.Date(2025, 3, 4, 5, 6, 7, 1, time.UTC),
		time.Date(0, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC),
		time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(-1, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Date(-10000, 6, 1, 0, 0, 0, 0, time.UTC),
		time.Unix(0, 0),
		time.Unix(1<<40, 999),
	}
	var out []time.Time
	for _, at := range instants {
		out = append(out, at)
		for _, z := range zones {
			out = append(out, at.In(z))
		}
	}
	return out
}

func requireTimeEncodingMatchesStd(t *testing.T, v any) {
	t.Helper()
	want, wantErr := stdjson.Marshal(v)
	got, gotErr := alosjson.Marshal(v)
	if (wantErr != nil) != (gotErr != nil) {
		t.Fatalf("Marshal(%v): std err=%v, alos err=%v (alos out %s)", v, wantErr, gotErr, got)
	}
	if wantErr != nil {
		return
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Marshal(%v) = %s, std = %s", v, got, want)
	}
	appended, err := alosjson.MarshalAppend([]byte("prefix"), v)
	if err != nil || !bytes.Equal(appended, append([]byte("prefix"), want...)) {
		t.Fatalf("MarshalAppend(%v) = %s, %v; want prefix%s", v, appended, err, want)
	}
	var w bytes.Buffer
	if err := alosjson.MarshalWrite(&w, v); err != nil || !bytes.Equal(bytes.TrimSpace(w.Bytes()), want) {
		t.Fatalf("MarshalWrite(%v) = %s, %v; want %s", v, w.Bytes(), err, want)
	}
}

func TestTimeEncodingMatchesStd(t *testing.T) {
	for _, at := range timeEdgeCases() {
		requireTimeEncodingMatchesStd(t, at)
		requireTimeEncodingMatchesStd(t, &at)
		requireTimeEncodingMatchesStd(t, timeHolder{At: at, Ptr: &at, OmitPtr: &at, List: []time.Time{at, at}, Map: map[string]time.Time{"k": at}, Any: at})
		requireTimeEncodingMatchesStd(t, []any{at, &at})
	}
}

func TestTimeEncodingNilPointers(t *testing.T) {
	var nilTime *time.Time
	requireTimeEncodingMatchesStd(t, nilTime)
	requireTimeEncodingMatchesStd(t, timeHolder{List: []time.Time{}, Map: map[string]time.Time{}})
	requireTimeEncodingMatchesStd(t, []*time.Time{nil, nil})
}

func TestTimeEncodingRespectsCustomMarshalers(t *testing.T) {
	at := time.Date(2025, 3, 4, 5, 6, 7, 0, time.UTC)
	custom := alosjson.MarshalFunc(func(v time.Time) ([]byte, error) {
		return []byte(`"custom"`), nil
	})
	got, err := alosjson.Marshal(timeHolder{At: at, Ptr: &at, List: []time.Time{at}}, alosjson.WithMarshalers(custom))
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"at":"custom","ptr":"2025-03-04T05:06:07Z","List":["custom"],"Map":{},"Any":null}`
	if string(got) != want {
		t.Fatalf("got %s, want %s", got, want)
	}
}

func TestTimeEncodingErrorMatchesMarshalJSON(t *testing.T) {
	at := time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)
	_, wantErr := at.MarshalJSON()
	_, gotErr := alosjson.Marshal(at)
	if wantErr == nil || gotErr == nil || gotErr.Error() != wantErr.Error() {
		t.Fatalf("alos err = %v, MarshalJSON err = %v", gotErr, wantErr)
	}
}

func TestTimeEncodingAllocations(t *testing.T) {
	if instrumentedBuild {
		t.Skip("allocation counts are not meaningful under race, asan or msan instrumentation")
	}
	at := time.Date(2025, 3, 4, 5, 6, 7, 123456789, time.UTC)
	holder := timeHolder{At: at, Ptr: &at}
	buf := make([]byte, 0, 512)
	allocs := testing.AllocsPerRun(allocRuns, func() {
		buf, _ = alosjson.MarshalAppend(buf[:0], &holder)
	})
	if allocs != 0 {
		t.Fatalf("MarshalAppend of a struct with time fields allocated %v times, want 0", allocs)
	}
}

func FuzzTimeEncodingDifferential(f *testing.F) {
	f.Add(int64(0), int64(0), int32(0))
	f.Add(int64(1741064767), int64(123456789), int32(19800))
	f.Add(int64(253402300800), int64(0), int32(0))
	f.Add(int64(-62135596801), int64(5), int32(-3600))
	f.Add(int64(1741064767), int64(1), int32(86400))
	f.Add(int64(1741064767), int64(1), int32(3601))
	f.Fuzz(func(t *testing.T, sec, nsec int64, offset int32) {
		at := time.Unix(sec, nsec).In(time.FixedZone("z", int(offset)))
		requireTimeEncodingMatchesStd(t, at)
		requireTimeEncodingMatchesStd(t, &at)
	})
}
