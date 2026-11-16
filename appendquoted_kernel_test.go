package json

import (
	"bytes"
	"strings"
	"testing"

	"github.com/guno1928/alos-json/jsontext"
)

const (
	kernelMaxTestLen  = 70
	kernelCanaryByte  = 0xA5
	kernelCanarySpare = 48
)

var kernelSpareCaps = []int{0, 1, 2, 5, 7, 8, 13, 31}

var kernelSampleBytes = []byte{'a', 0x00, 0x01, '\b', '\t', '\n', '\f', '\r', 0x1F, '"', '\\', 0x20, '<', '>', '&', 0x7F, 0x80, 0xE2, 0xFF}

func checkAppendQuotedKernel(t *testing.T, s string) {
	t.Helper()
	want := appendQuotedPortable([]byte("pre"), s)
	for _, spare := range kernelSpareCaps {
		dst := append(make([]byte, 0, len("pre")+spare), "pre"...)
		if got := appendQuotedKernel(dst, s); !bytes.Equal(got, want) {
			t.Fatalf("appendQuotedKernel(%q, spare=%d) = %q, want %q", s, spare, got, want)
		}
		if got := appendJStr(dst, s); !bytes.Equal(got, want) {
			t.Fatalf("appendJStr(%q, spare=%d) = %q, want %q", s, spare, got, want)
		}
	}
	buf := bytes.Repeat([]byte{kernelCanaryByte}, len(want)+kernelCanarySpare)
	got := appendQuotedKernel(append(buf[:0], "pre"...), s)
	if !bytes.Equal(got, want) {
		t.Fatalf("appendQuotedKernel(%q) into roomy buffer = %q, want %q", s, got, want)
	}
	for i, c := range buf[len(want):] {
		if c != kernelCanaryByte {
			t.Fatalf("appendQuotedKernel(%q) wrote past its result at byte %d", s, len(want)+i)
		}
	}
}

func TestAppendQuotedKernelMatchesPortable(t *testing.T) {
	checkAppendQuotedKernel(t, "")
	for n := 1; n <= kernelMaxTestLen; n++ {
		for _, filler := range kernelSampleBytes {
			base := bytes.Repeat([]byte{filler}, n)
			checkAppendQuotedKernel(t, string(base))
			for pos := range n {
				for c := range 256 {
					if filler != 'a' && c%17 != 0 {
						continue
					}
					b := append([]byte(nil), base...)
					b[pos] = byte(c)
					checkAppendQuotedKernel(t, string(b))
				}
			}
		}
	}
}

func TestAppendQuotedKernelRealisticText(t *testing.T) {
	for _, s := range []string{
		`He said "hello" and left.`,
		"tab\tseparated\tvalues",
		"line one\nline two\r\n",
		`C:\Users\path\to\file`,
		"<script>alert('x')</script> & more",
		"café naïve résumé",
		"日本語のテキストです",
		"emoji 😀🎉 party",
		"控制字符\u0001\u001f",
		"\u2028\u2029 separators",
		"\xff\xfe invalid \xc3\x28 utf8",
	} {
		for reps := 1; reps <= 40; reps++ {
			checkAppendQuotedKernel(t, strings.Repeat(s+" ", reps))
		}
	}
	checkAppendQuotedKernel(t, strings.Repeat("x", 1<<16+3))
	checkAppendQuotedKernel(t, strings.Repeat("\"", 1<<12+5))
}

func TestMarshalStringsMatchReferenceQuoters(t *testing.T) {
	inputs := []string{"", "a", "<&>", `"q"`, "\u2028", "\x01", "plain text", strings.Repeat("<a\n", 50)}
	for _, s := range inputs {
		plain := append(appendQuotedPortable([]byte("["), s), ']')
		html := append(jsontext.AppendQuoteStringHTML([]byte("["), s), ']')
		for _, tc := range []struct {
			opts []Option
			want []byte
		}{{nil, plain}, {[]Option{EscapeHTML(false)}, plain}, {[]Option{EscapeHTML(true)}, html}} {
			got, err := Marshal([]string{s}, tc.opts...)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, tc.want) {
				t.Fatalf("Marshal(%q, %d opts) = %q, want %q", s, len(tc.opts), got, tc.want)
			}
		}
	}
}

func FuzzAppendQuotedKernel(f *testing.F) {
	f.Add([]byte(`He said "hello" and left.`), uint8(0))
	f.Add([]byte("控制字符\x01\x1f"), uint8(3))
	f.Add(bytes.Repeat([]byte{'\\'}, 90), uint8(7))
	f.Fuzz(func(t *testing.T, data []byte, spare uint8) {
		s := string(data)
		want := appendQuotedPortable([]byte("pre"), s)
		dst := append(make([]byte, 0, len("pre")+int(spare)), "pre"...)
		if got := appendQuotedKernel(dst, s); !bytes.Equal(got, want) {
			t.Fatalf("appendQuotedKernel(%q) = %q, want %q", s, got, want)
		}
	})
}
