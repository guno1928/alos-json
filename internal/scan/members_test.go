package scan

import (
	"strconv"
	"strings"
	"testing"
)

const unlimitedBudget = 1 << 30

func membersAfterBrace(s string, budget int) (int, int) {
	b := []byte(s)
	return ObjectMembers(b, SkipSpace(b, 1), budget)
}

func TestObjectMembers(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
	}{
		{`{}`, 0},
		{`{"a":1}`, 1},
		{`{ "a" : 1 , "b" : [1,{"x":2}] , "c":"}\"," }`, 3},
		{`{"a":{"b":{"c":{}}},"d":null,"e":true,"f":-1.5e3}`, 4},
		{`{"d":null,"e":true,"f":false,"g":"é\né","h":[]}`, 5},
		{"{\n\t\"a\" :\t1,\r\n\"b\":2}", 2},
		{`{"a":1,"a":2}`, 2},
		{`{"a":1,`, 1},
		{`{"a":1 "b":2}`, 1},
		{`{"a":}`, 0},
		{`{"a"1}`, 0},
		{`{"a`, 0},
		{`{1:2}`, 0},
		{`{"a":[1,2`, 0},
		{`{"a":1,"b":[1,2`, 1},
	} {
		got, end := membersAfterBrace(tc.in, unlimitedBudget)
		if got != tc.want {
			t.Errorf("ObjectMembers(%s) = %d, want %d", tc.in, got, tc.want)
		}
		if end < 1 || end > len(tc.in) {
			t.Errorf("ObjectMembers(%s) end %d outside input", tc.in, end)
		}
	}
}

func TestObjectMembersEndsAtObjectClose(t *testing.T) {
	in := `{"a":[1,{"b":2}],"c":"x"} trailing`
	count, end := membersAfterBrace(in, unlimitedBudget)
	if count != 2 || in[end] != '}' {
		t.Fatalf("count %d end %d (%q), want 2 members ending at the closing brace", count, end, in[end:])
	}
}

func TestObjectMembersBudget(t *testing.T) {
	var sb strings.Builder
	sb.WriteByte('{')
	const members = 1000
	for i := range members {
		if i > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(`"k`)
		sb.WriteString(strconv.Itoa(i))
		sb.WriteString(`":`)
		sb.WriteString(strconv.Itoa(i))
	}
	sb.WriteByte('}')
	in := sb.String()
	if got, _ := membersAfterBrace(in, unlimitedBudget); got != members {
		t.Fatalf("unlimited budget: %d members, want %d", got, members)
	}
	small, end := membersAfterBrace(in, 100)
	if small < 1 || small >= members || end > 200 {
		t.Fatalf("budget 100: %d members ending at %d, want a partial count that stops early", small, end)
	}
	if got, _ := membersAfterBrace(in, 0); got != 1 {
		t.Fatalf("budget 0: %d members, want 1", got)
	}
}
