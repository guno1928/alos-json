//go:build amd64 && !purego

package scan

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
)

func indexedStrict(b []byte) bool {
	return validIndexed(b)
}

func exerciseIndexedValid(b []byte) {
	if useValidKernel {
		indexedStrict(b)
		validAdaptive(b)
	}
}

func requireValidKernel(t testing.TB) {
	t.Helper()
	if !useValidKernel {
		t.Skip("CPU lacks AVX2, BMI1/2, PCLMULQDQ or POPCNT")
	}
}

func checkIndexed(t testing.TB, b []byte) {
	t.Helper()
	want := json.Valid(b)
	if got := indexedStrict(b); got != want {
		t.Fatalf("validIndexed strict (len %d) %q = %v, encoding/json.Valid = %v", len(b), b, got, want)
	}
	if got := validAdaptive(b); got != want {
		t.Fatalf("validAdaptive (len %d) %q = %v, encoding/json.Valid = %v", len(b), b, got, want)
	}
	if got := Valid(b); got != want {
		t.Fatalf("Valid (len %d) %q = %v, encoding/json.Valid = %v", len(b), b, got, want)
	}
	wantLax := laxSpec(b)
	if got := validIndexedMode(b, true); got != wantLax {
		t.Fatalf("validIndexed lax (len %d) %q = %v, spec = %v", len(b), b, got, wantLax)
	}
	if got := validAdaptiveLax(b); got != wantLax {
		t.Fatalf("validAdaptive lax (len %d) %q = %v, spec = %v", len(b), b, got, wantLax)
	}
	if got := ValidLax(b); got != wantLax {
		t.Fatalf("ValidLax (len %d) %q = %v, spec = %v", len(b), b, got, wantLax)
	}
}

var indexedOffsets = func() []int {
	offsets := []int{0, 1, 2, 7, 30}
	for o := 55; o <= 72; o++ {
		offsets = append(offsets, o)
	}
	return append(offsets, 119, 120, 121, 122, 123, 124, 125, 126, 127, 128, 129)
}()

func TestValidIndexedEveryByteInShortStrings(t *testing.T) {
	requireValidKernel(t)
	const base = "abcdefg"
	for size := 1; size <= len(base); size++ {
		for pos := range size {
			for v := range 256 {
				content := []byte(base[:size])
				content[pos] = byte(v)
				escaped := append([]byte(nil), content...)
				escaped[pos] = '\\'
				if pos+1 < size {
					escaped[pos+1] = byte(v)
				}
				for _, s := range [][]byte{content, escaped} {
					for _, off := range indexedOffsets {
						pad := strings.Repeat(" ", off)
						checkIndexed(t, []byte(pad+`["`+string(s)+`"]`))
						checkIndexed(t, []byte(pad+`{"`+string(s)+`":1}`))
						checkIndexed(t, []byte(pad+`"`+string(s)+`"`))
					}
				}
			}
		}
	}
}

var backslashFollowers = []string{`"`, `\`, `/`, `b`, `n`, `t`, `u0041`, `uD83D\uDE00`, `uabcF`, `u12`, `u12g4`, `x`, `U0041`, "\x01", "\x7f", "é", ``}

func TestValidIndexedBackslashRunsAcrossBlocks(t *testing.T) {
	requireValidKernel(t)
	for run := 0; run <= 70; run++ {
		slashes := strings.Repeat(`\`, run)
		for off := 0; off <= 135; off++ {
			lead := strings.Repeat("a", off)
			for _, f := range backslashFollowers {
				body := `"` + lead + slashes + f
				checkIndexed(t, []byte(`[`+body+`"]`))
				checkIndexed(t, []byte(`[`+body+`","z",1]`))
				checkIndexed(t, []byte(body+`"`))
				checkIndexed(t, []byte(body))
			}
		}
	}
}

func indexedCorpus() [][]byte {
	docs := [][]byte{
		[]byte(`[` + strings.Repeat("1,", 3000) + `1]`),
		[]byte(strings.Repeat(`{"a":[`, 300) + `"x"` + strings.Repeat(`]}`, 300)),
		[]byte(`{"t":"` + strings.Repeat("long text ", 400) + `","e":"` + strings.Repeat(`\"\\\/\b\f\n\r\té`, 60) + `"}`),
		[]byte(` [ -0.5e+10 , 1E-2 , 0 , -0 , 12345678901234567890 , true , false , null , "" , { } , [ ] ] `),
	}
	g := &docGen{x: 0xA4093822299F31D0}
	for range 120 {
		var sb strings.Builder
		g.value(&sb, 0)
		docs = append(docs, []byte(sb.String()))
	}
	next := xorshiftBytes(0x082EFA98EC4E6C89)
	for range 120 {
		docs = append(docs, randomEscapedDoc(next))
	}
	return docs
}

func TestValidIndexedEveryTruncation(t *testing.T) {
	requireValidKernel(t)
	for _, doc := range indexedCorpus() {
		for cut := 0; cut <= len(doc); cut++ {
			checkIndexed(t, doc[:cut])
			checkIndexed(t, doc[len(doc)-cut:])
		}
	}
}

func TestValidIndexedRandomDocsAndMutations(t *testing.T) {
	requireValidKernel(t)
	g := &docGen{x: 0x452821E638D01377}
	for range 6_000 {
		var sb strings.Builder
		wrappers := 1 + int(g.next()%4)
		for range wrappers {
			sb.WriteString(`{"pad":"` + strings.Repeat("p", int(g.next()%90)) + `","v":`)
		}
		g.value(&sb, 0)
		doc := []byte(sb.String() + strings.Repeat("}", wrappers))
		checkIndexed(t, doc)
		for range 12 {
			m := append([]byte(nil), doc...)
			pos := int(g.next() % uint64(len(m)))
			switch g.next() % 3 {
			case 0:
				m[pos] = mutationBytes[g.next()%uint64(len(mutationBytes))]
			case 1:
				m = append(m[:pos], m[pos+1:]...)
			default:
				m = append(m[:pos], append([]byte{mutationBytes[g.next()%uint64(len(mutationBytes))]}, m[pos:]...)...)
			}
			checkIndexed(t, m)
		}
	}
}

func TestValidIndexedDepthLimits(t *testing.T) {
	requireValidKernel(t)
	for _, d := range []int{1, 63, 64, 65, 127, 128, 129, 1023, 1024, 1025, MaxDepth - 1, MaxDepth, MaxDepth + 1} {
		arr := strings.Repeat("[", d) + strings.Repeat("]", d)
		obj := strings.Repeat(`{"k":`, d) + "1" + strings.Repeat("}", d)
		mixed := strings.Repeat(`[{"a":`, d/2+1) + "0" + strings.Repeat("}]", d/2+1)
		for _, doc := range []string{arr, obj, mixed, arr + " ", " " + obj, arr[:len(arr)-1], obj + "}", arr[:len(arr)-1] + "}"} {
			checkIndexed(t, []byte(doc))
		}
	}
}

func blockEndingDocs(size int) []string {
	fill := func(n int, c string) string { return strings.Repeat(c, max(n, 0)) }
	docs := []string{
		`"` + fill(size-2, "x") + `"`,
		`"` + fill(size-3, "x") + `\n"`,
		`"` + fill(size-8, "x") + `é"`,
		`"` + fill(size-2, "x") + `\`,
		`"` + fill(size-1, "x"),
		`"` + fill(size-3, "x") + `\"`,
		`"` + fill(size-4, "x") + `\\"`,
		`"` + fill(size-6, "x") + `\u12"`,
		fill(size, "1"),
		"-" + fill(size-1, "2"),
		"0." + fill(size-2, "5"),
		"1e" + fill(size-2, "9"),
		fill(size-4, " ") + "true",
		fill(size-5, " ") + "false",
		fill(size-4, " ") + "null",
		fill(size-3, " ") + "nul",
		fill(size-1, " ") + "1",
		`[` + fill(size-2, " ") + `]`,
		`{"k":` + fill(size-7, " ") + `1}`,
		`["` + fill(size-4, "y") + `"]`,
		fill(size-2, " ") + `""`,
		fill(size-1, " ") + `"`,
		`[` + strings.TrimSuffix(fill((size-1)/2, "0,"), ",") + `]`,
	}
	return docs
}

func TestValidIndexedInputsEndingAtBlockBoundaries(t *testing.T) {
	requireValidKernel(t)
	for size := 1; size <= 3*BlockLen+2; size++ {
		for _, doc := range blockEndingDocs(size) {
			checkIndexed(t, []byte(doc))
			checkIndexed(t, []byte(doc+" "))
			checkIndexed(t, []byte(doc+"\n\t"))
		}
	}
}

func TestValidIndexedLongStrings(t *testing.T) {
	requireValidKernel(t)
	for _, n := range []int{64, 100, 1000, 4096, 70_000} {
		body := strings.Repeat("abcdefgh ", n/9+1)[:n]
		doc := `{"title":"x","body":"` + body + `"}`
		checkIndexed(t, []byte(doc))
		for _, c := range []byte{0x00, 0x1f, '"', '\\', '\t', 0x7f, 0xff} {
			for _, pos := range []int{0, n / 2, n - 1} {
				m := []byte(doc)
				m[len(`{"title":"x","body":"`)+pos] = c
				checkIndexed(t, m)
			}
		}
	}
}

func TestValidIndexedManyTokens(t *testing.T) {
	requireValidKernel(t)
	var sb strings.Builder
	sb.WriteByte('[')
	for i := range 20_000 {
		if i > 0 {
			sb.WriteByte(',')
		}
		switch i % 5 {
		case 0:
			sb.WriteString(strconv.Itoa(i * 7919))
		case 1:
			sb.WriteString(`"s` + strconv.Itoa(i) + `"`)
		case 2:
			sb.WriteString(`{"k":[true,null]}`)
		case 3:
			sb.WriteString("-1.5e3")
		default:
			sb.WriteString("[]")
		}
	}
	sb.WriteByte(']')
	doc := []byte(sb.String())
	checkIndexed(t, doc)
	for _, pos := range []int{1, len(doc) / 3, len(doc) / 2, len(doc) - 2} {
		for _, c := range []byte{'x', ',', ']', '"', ':', '\\'} {
			m := append([]byte(nil), doc...)
			m[pos] = c
			checkIndexed(t, m)
		}
	}
}

func TestValidKernelFallbackMatches(t *testing.T) {
	saved := useValidKernel
	defer func() { useValidKernel = saved }()
	for _, doc := range indexedCorpus() {
		useValidKernel = false
		off, offLax := Valid(doc), ValidLax(doc)
		useValidKernel = saved
		if on, want := Valid(doc), json.Valid(doc); off != want || on != want {
			t.Fatalf("Valid(%q): kernel %v, fallback %v, encoding/json %v", doc, on, off, want)
		}
		if on, want := ValidLax(doc), laxSpec(doc); offLax != want || on != want {
			t.Fatalf("ValidLax(%q): kernel %v, fallback %v, spec %v", doc, on, offLax, want)
		}
	}
}

func FuzzValidIndexed(f *testing.F) {
	for _, doc := range indexedCorpus() {
		if len(doc) < 4096 {
			f.Add(doc)
		}
	}
	f.Fuzz(func(t *testing.T, doc []byte) {
		requireValidKernel(t)
		checkIndexed(t, doc)
	})
}
