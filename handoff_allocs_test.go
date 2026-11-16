package json_test

import (
	"strings"
	"testing"

	alosjson "github.com/guno1928/alos-json"
)

const largeStringAllocLen = 1 << 20

func TestMarshalLargeStringAllocations(t *testing.T) {
	if instrumentedBuild {
		t.Skip("allocation counts are not meaningful under race, asan or msan instrumentation")
	}
	doc := struct {
		Title string `json:"title"`
		Body  string `json:"body"`
	}{Title: "A long document", Body: strings.Repeat("w", largeStringAllocLen)}
	if got := testing.AllocsPerRun(allocRuns, func() { _, _ = alosjson.Marshal(&doc) }); got > 1 {
		t.Fatalf("Marshal of a large string: %v allocs/op, want at most 1", got)
	}
}
