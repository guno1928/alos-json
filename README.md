# alos-json

[![Go Reference](https://pkg.go.dev/badge/github.com/guno1928/alos-json.svg)](https://pkg.go.dev/github.com/guno1928/alos-json)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A fast JSON library for Go. The API follows `encoding/json` (`Marshal`, `Unmarshal`, `Valid`, …), and the defaults and options follow `encoding/json/v2`. On amd64 the hot loops use SSE2 and AVX2 assembly, with AVX2 picked at run time. Every other platform gets the same results from pure Go.

```sh
go get github.com/guno1928/alos-json
```

Requires Go 1.24 or newer. The package name is `json`.

## At a glance

Geometric mean over the datasets in [`bench/`](bench), measured on an Intel Xeon E3-1270 v6 with Go 1.26.11 on Linux. Each number is how many times longer the other library takes than alos, so **above 1.00 means alos is faster** and below 1.00 means the other library is faster.

| Operation | encoding/json | json/v2 | sonic | sonic (std) | goccy | jsoniter | segmentio | alos fastest on |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Unmarshal into structs | 5.22 | 4.12 | 1.44 | 1.80 | 1.45 | 2.80 | 2.08 | 13 / 13 |
| Marshal structs | 4.22 | 4.20 | 1.24 | 1.55 | 1.36 | 3.20 | 2.22 | 12 / 13 |
| Unmarshal into `any` | 5.55 | 2.83 | 1.32 | 1.56 | 1.40 | 3.11 | 3.75 | 7 / 7 |
| Marshal `any` | 3.82 | 3.09 | 1.65 | 1.99 | 1.72 | 3.02 | 2.88 | 7 / 7 |
| Valid | 3.43 | 4.24 | 1.90 | 1.90 | 1.87 | 6.07 | 2.07 | 10 / 13 |

alos is faster than every other library on the geometric mean of all five operations, and is the fastest on 49 of the 53 rows. sonic is 0.3% faster at encoding `twitter_100`. Of the Valid rows, goccy is faster on `large_100` and `tree_3280` (by 1.9% and 0.8%) and sonic on `escaped_strings`. sonic's Valid does not check escape sequences or control characters inside strings, so it accepts some input that `encoding/json` and alos reject. Full tables are in [Benchmarks](#benchmarks).

## Quick start

```go
package main

import (
	"fmt"

	json "github.com/guno1928/alos-json"
)

type User struct {
	ID    int64    `json:"id"`
	Name  string   `json:"name"`
	Email string   `json:"email,omitempty"`
	Tags  []string `json:"tags"`
}

func main() {
	data, err := json.Marshal(User{ID: 1, Name: "Ada", Tags: []string{"admin"}})
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))

	var u User
	if err := json.Unmarshal([]byte(`{"id":2,"name":"Grace","tags":[]}`), &u); err != nil {
		panic(err)
	}
	fmt.Println(u.Name, json.Valid(data))
}
```

Output:

```
{"id":1,"name":"Ada","tags":["admin"]}
Grace true
```

### Reusing a buffer

`MarshalAppend` appends to a slice you own. When the slice has room it does not allocate.

```go
buf := make([]byte, 0, 1024)
for _, u := range users {
	buf, err = json.MarshalAppend(buf[:0], u)
	if err != nil {
		return err
	}
	send(buf)
}
```

### Readers and writers

`json.MarshalWrite(w, v)` and `json.UnmarshalRead(r, &v)` work with `io.Writer` and `io.Reader`. The [`jsontext`](jsontext) package has a token-level `Encoder` and `Decoder` for streaming.

### Custom types

Types with `MarshalJSON() ([]byte, error)` and `UnmarshalJSON([]byte) error` methods are used automatically, the same as with `encoding/json`.

To change how a type is encoded without adding methods to it, pass functions as options. They follow `encoding/json/v2`:

```go
type Celsius float64

asText := json.MarshalFunc(func(c Celsius) ([]byte, error) {
	return []byte(strconv.Quote(strconv.FormatFloat(float64(c), 'f', 1, 64) + "C")), nil
})
data, err := json.Marshal(map[string]Celsius{"office": 21.5}, json.WithMarshalers(asText))
```

`data` is `{"office":"21.5C"}`.

| Function | Use |
|---|---|
| `MarshalFunc(func(T) ([]byte, error))` | Return the JSON for a `T`. |
| `MarshalToFunc(func(*jsontext.Encoder, T) error)` | Write a `T` to the encoder, or return `SkipFunc` to use the default. |
| `UnmarshalFunc(func([]byte, T) error)` | Decode the raw JSON value into `T`, which must be a pointer or interface type. |
| `UnmarshalFromFunc(func(*jsontext.Decoder, T) error)` | Read the value from the decoder, or return `SkipFunc` to use the default. |
| `JoinMarshalers` / `JoinUnmarshalers` | Combine several. The first one that applies wins. |

`T` can be an interface: `MarshalFunc(func(fmt.Stringer) ([]byte, error))` applies to every type whose pointer implements `fmt.Stringer`. The functions take priority over `MarshalJSON` methods and also apply to map keys, the same as `encoding/json/v2`. A `Marshalers` value caches its encoders, so build it once and reuse it rather than creating it for every call. `UnmarshalRead` reads the whole input before decoding when unmarshal functions are set.

## API

| Function | Description |
|---|---|
| `Marshal(v, opts...)` | Encode `v` as JSON. |
| `MarshalAppend(dst, v, opts...)` | Encode `v` and append it to `dst`. |
| `MarshalWrite(w, v, opts...)` | Encode `v` and write it to `w`. |
| `Unmarshal(data, &v, opts...)` | Decode `data` into `v`. |
| `UnmarshalRead(r, &v, opts...)` | Decode JSON read from `r` into `v`. |
| `Valid(data)` | Report whether `data` is valid JSON. |
| `Compact(&dst, src)` / `Indent(&dst, src, prefix, indent)` | Reformat existing JSON. |

## Options

Options are passed as trailing arguments, for example `json.Marshal(v, json.Deterministic(true))`.

| Option | Applies to | Effect |
|---|---|---|
| `Deterministic(true)` | Marshal | Sort map keys. Without it, map keys come out in Go's map iteration order. |
| `EscapeHTML(true)` | Marshal | Escape `<`, `>`, `&`, U+2028 and U+2029 as `\u003c`, `\u003e`, `\u0026`, `\u2028`, `\u2029`. |
| `FormatNilSliceAsNull(true)` | Marshal | Encode nil slices as `null` instead of `[]`. |
| `FormatNilMapAsNull(true)` | Marshal | Encode nil maps as `null` instead of `{}`. |
| `WithIndent(prefix, indent)` | Marshal | Pretty-print. Same layout as `encoding/json.MarshalIndent`. `WithIndent("", "")` keeps the output compact. |
| `OmitZeroStructFields(true)` | Marshal | Omit every struct field that is the zero value, as if each had `omitzero`. |
| `StringifyNumbers(true)` | Marshal | Encode Go numbers as JSON strings, for example `"42"`. Decoding accepts quoted numbers with or without it. |
| `WithMarshalers(m)` | Marshal | Use the functions in `m` for their types. See [Custom types](#custom-types). |
| `MatchCaseInsensitiveNames(true)` | Unmarshal | Match object keys to struct fields without regard to case. |
| `RejectUnknownMembers(true)` | Unmarshal | Return an error for object keys that match no struct field and that no inline map can hold. |
| `WithUnmarshalers(u)` | Unmarshal | Use the functions in `u` for their types. |

To get the same output as `encoding/json`:

```go
data, err := json.Marshal(v,
	json.Deterministic(true),
	json.EscapeHTML(true),
	json.FormatNilSliceAsNull(true),
	json.FormatNilMapAsNull(true),
)
```

## Struct tags

| Tag | Effect |
|---|---|
| `json:"name"` | Use `name` as the object key. |
| `json:"-"` | Skip the field. |
| `json:",omitempty"` | Omit the field when it is false, 0, a nil pointer or interface, or an empty string, slice or map. |
| `json:",omitzero"` | Omit the field when it is the zero value, or when its `IsZero() bool` method returns true. |
| `json:",string"` | Encode a number or bool inside a JSON string, and accept it that way when decoding. |
| `json:",inline"` | On a struct or struct pointer field, promote its fields into the parent object. On a `map[string]T` field, collect object keys that match no other field when decoding, and write the map's entries into the object when encoding. |
| `json:",nocase"` | Match this field's key without regard to case when decoding. An exact match on another field still wins. |

Fields of embedded structs are promoted into the parent object, as in `encoding/json`. A nil struct pointer marked `inline` adds nothing when encoding and is allocated when decoding needs it. Encoding fails if an inline map holds a key that one of the struct's encoded fields already used.

Current `encoding/json/v2` spells `nocase` as `case:ignore`. alos accepts only `nocase`.

## Differences from encoding/json

| | alos (default) | encoding/json | How to change |
|---|---|---|---|
| nil slice | `[]` | `null` | `FormatNilSliceAsNull(true)` |
| nil map | `{}` | `null` | `FormatNilMapAsNull(true)` |
| `<`, `>`, `&`, U+2028, U+2029 in strings | written as is | escaped | `EscapeHTML(true)` |
| map key order | not sorted | sorted | `Deterministic(true)` |
| object key matching | exact | case-insensitive | `MatchCaseInsensitiveNames(true)` |
| invalid UTF-8 in strings | kept byte for byte | replaced with U+FFFD | – |
| `3.9` decoded into an integer | `3` | error | – |
| `"42"` decoded into an integer | `42` | error | – |
| `"true"` decoded into a bool | `true` | error | – |

Behaviour that is the same:

- Unknown object keys are ignored unless you ask for an error.
- Duplicate keys keep the last value.
- Numbers that don't fit the target type (`300` into an `int8`) are an error.
- Nesting deeper than 10,000 levels is rejected.
- Floats are written in the same format: shortest round-trip digits, with exponent form below 1e-6 and from 1e21 up.

## Platform support

- **amd64:** SSE2 assembly for string scanning and quoting. AVX2 assembly for long strings and escape-heavy strings, used only when the CPU reports AVX2.
- **Everything else:** pure Go. The test suite has been run on arm64 and on big-endian s390x (both under QEMU), and the library builds for 386, arm, mips and riscv64.
- **`-tags purego`:** builds without any assembly.

## Benchmarks

### How they were measured

- **Machine:** Intel Xeon E3-1270 v6 @ 3.80GHz (4 cores, 8 threads), Linux, Go 1.26.11 in the `golang:1.26` Docker image, amd64 with AVX2.
- **Runs:** each benchmark ran for 150 ms, 6 times; the tables show the median.
- **json/v2:** measured with `GOEXPERIMENT=jsonv2`.
- **Library versions:** sonic v1.15.5 (`sonic` is the default config, `sonic (std)` is `sonic.ConfigStd`), goccy/go-json v0.12.0, json-iterator/go v1.1.12, segmentio/encoding v0.5.4.

The fastest result in each row is in bold.

| Dataset | Contents |
|---|---|
| `small` | One struct with four fields. |
| `medium` | One user record with nested address, tags and numbers. |
| `large_100` / `xlarge_10k` | 100 / 10,000 user records. |
| `short_strings` | 2,000 short words. |
| `long_string_64k` / `long_string_1m` | One 64 KiB / 1 MiB string. |
| `escaped_strings` | 500 strings full of quotes, tabs, newlines, backslashes, HTML characters, non-ASCII text and control characters. |
| `ints_10k` / `floats_10k` | 10,000 `int64` / `float64` values. |
| `twitter_100` | 100 tweet-like objects. |
| `tree_3280` | A tree of 3,280 nodes (depth 7, fan-out 3). |
| `deep_chain_200` | 200 nested objects. |

To run them yourself:

```sh
cd bench
go test -run '^$' -bench . -benchtime 150ms -count 3
GOEXPERIMENT=jsonv2 go test -run '^$' -bench . -benchtime 150ms -count 3
```

The second command includes json/v2.

### Unmarshal into structs

| dataset | alos | encoding/json | json/v2 | sonic | sonic (std) | goccy | jsoniter | segmentio |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| small | **211 ns** | 761 ns | 611 ns | 312 ns | 380 ns | 223 ns | 314 ns | 294 ns |
| medium | **500 ns** | 2.2 µs | 1.8 µs | 820 ns | 1.0 µs | 648 ns | 955 ns | 855 ns |
| large_100 | **46.8 µs** | 213 µs | 188 µs | 78.1 µs | 92.5 µs | 62.2 µs | 90.5 µs | 85.8 µs |
| xlarge_10k | **4.94 ms** | 21.76 ms | 19.32 ms | 8.03 ms | 9.92 ms | 6.72 ms | 10.25 ms | 8.42 ms |
| short_strings | **36.0 µs** | 194 µs | 171 µs | 41.0 µs | 74.1 µs | 70.7 µs | 91.0 µs | 78.8 µs |
| long_string_64k | **10.9 µs** | 76.2 µs | 42.8 µs | 11.1 µs | 21.5 µs | 12.8 µs | 58.9 µs | 12.5 µs |
| long_string_1m | **147 µs** | 1.18 ms | 632 µs | 189 µs | 333 µs | 217 µs | 894 µs | 182 µs |
| escaped_strings | **77.0 µs** | 535 µs | 408 µs | 85.1 µs | 91.0 µs | 184 µs | 398 µs | 475 µs |
| ints_10k | **162 µs** | 665 µs | 512 µs | 197 µs | 196 µs | 186 µs | 298 µs | 198 µs |
| floats_10k | **204 µs** | 1.46 ms | 1.24 ms | 370 µs | 368 µs | 266 µs | 1.71 ms | 798 µs |
| twitter_100 | **222 µs** | 832 µs | 702 µs | 282 µs | 305 µs | 312 µs | 508 µs | 623 µs |
| tree_3280 | **464 µs** | 2.53 ms | 2.21 ms | 887 µs | 952 µs | 722 µs | 936 µs | 1.03 ms |
| deep_chain_200 | **25.7 µs** | 138 µs | 123 µs | 49.1 µs | 54.4 µs | 47.1 µs | 47.8 µs | 80.6 µs |

### Marshal structs

| dataset | alos | encoding/json | json/v2 | sonic | sonic (std) | goccy | jsoniter | segmentio |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| small | **125 ns** | 425 ns | 426 ns | 182 ns | 255 ns | 150 ns | 303 ns | 198 ns |
| medium | **242 ns** | 920 ns | 928 ns | 314 ns | 417 ns | 267 ns | 587 ns | 385 ns |
| large_100 | **18.2 µs** | 81.7 µs | 82.5 µs | 21.4 µs | 22.6 µs | 21.6 µs | 50.4 µs | 30.9 µs |
| xlarge_10k | **1.98 ms** | 8.50 ms | 8.56 ms | 3.48 ms | 3.90 ms | 2.11 ms | 5.13 ms | 3.33 ms |
| short_strings | **20.2 µs** | 62.4 µs | 62.3 µs | 24.5 µs | 25.7 µs | 24.9 µs | 32.1 µs | 32.3 µs |
| long_string_64k | **10.6 µs** | 57.6 µs | 57.9 µs | 12.6 µs | 17.4 µs | 12.0 µs | 96.3 µs | 32.0 µs |
| long_string_1m | **141 µs** | 910 µs | 903 µs | 168 µs | 424 µs | 218 µs | 1.49 ms | 561 µs |
| escaped_strings | **35.0 µs** | 137 µs | 126 µs | 39.0 µs | 60.9 µs | 56.8 µs | 138 µs | 156 µs |
| ints_10k | **53.2 µs** | 224 µs | 224 µs | 88.5 µs | 94.8 µs | 135 µs | 131 µs | 135 µs |
| floats_10k | **317 µs** | 861 µs | 874 µs | 378 µs | 390 µs | 524 µs | 731 µs | 725 µs |
| twitter_100 | 89.6 µs | 357 µs | 344 µs | **89.3 µs** | 100.0 µs | 98.3 µs | 234 µs | 152 µs |
| tree_3280 | **205 µs** | 991 µs | 1.00 ms | 225 µs | 235 µs | 316 µs | 609 µs | 434 µs |
| deep_chain_200 | **10.1 µs** | 58.1 µs | 59.0 µs | 10.9 µs | 11.6 µs | 13.0 µs | 34.0 µs | 25.2 µs |

### Unmarshal into `any`

| dataset | alos | encoding/json | json/v2 | sonic | sonic (std) | goccy | jsoniter | segmentio |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| small | **456 ns** | 2.0 µs | 988 ns | 559 ns | 703 ns | 551 ns | 819 ns | 834 ns |
| medium | **1.1 µs** | 5.8 µs | 2.6 µs | 1.5 µs | 1.9 µs | 1.6 µs | 2.8 µs | 2.5 µs |
| large_100 | **107 µs** | 572 µs | 258 µs | 135 µs | 180 µs | 150 µs | 283 µs | 294 µs |
| escaped_strings | **78.5 µs** | 623 µs | 418 µs | 89.4 µs | 94.2 µs | 206 µs | 455 µs | 802 µs |
| floats_10k | **288 µs** | 2.72 ms | 1.47 ms | 592 µs | 597 µs | 384 µs | 2.59 ms | 1.71 ms |
| twitter_100 | **561 µs** | 2.11 ms | 1.09 ms | 613 µs | 745 µs | 581 µs | 1.14 ms | 1.71 ms |
| tree_3280 | **1.29 ms** | 6.22 ms | 2.93 ms | 1.71 ms | 1.99 ms | 1.56 ms | 2.89 ms | 6.56 ms |

### Marshal `any`

| dataset | alos | encoding/json | json/v2 | sonic | sonic (std) | goccy | jsoniter | segmentio |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| small | **233 ns** | 1.6 µs | 1.4 µs | 437 ns | 554 ns | 375 ns | 627 ns | 531 ns |
| medium | **520 ns** | 3.4 µs | 2.6 µs | 988 ns | 1.2 µs | 891 ns | 1.4 µs | 1.5 µs |
| large_100 | **46.7 µs** | 146 µs | 106 µs | 90.1 µs | 104 µs | 85.1 µs | 140 µs | 146 µs |
| escaped_strings | **34.7 µs** | 138 µs | 125 µs | 42.2 µs | 64.1 µs | 60.1 µs | 150 µs | 163 µs |
| floats_10k | **350 µs** | 897 µs | 893 µs | 473 µs | 487 µs | 560 µs | 965 µs | 881 µs |
| twitter_100 | **215 µs** | 650 µs | 452 µs | 359 µs | 418 µs | 372 µs | 612 µs | 609 µs |
| tree_3280 | **628 µs** | 1.73 ms | 1.31 ms | 1.10 ms | 1.26 ms | 1.14 ms | 1.92 ms | 1.52 ms |

### Valid

| dataset | alos | encoding/json | json/v2 | sonic | sonic (std) | goccy | jsoniter | segmentio |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| small | **51 ns** | 177 ns | 246 ns | 134 ns | 134 ns | 80 ns | 239 ns | 89 ns |
| medium | **147 ns** | 390 ns | 558 ns | 382 ns | 383 ns | 175 ns | 639 ns | 248 ns |
| large_100 | 15.1 µs | 31.3 µs | 47.1 µs | 34.8 µs | 34.9 µs | **14.8 µs** | 61.5 µs | 24.1 µs |
| xlarge_10k | **1.55 ms** | 3.05 ms | 4.71 ms | 3.49 ms | 3.47 ms | 1.59 ms | 6.16 ms | 2.48 ms |
| short_strings | **12.1 µs** | 26.0 µs | 25.9 µs | 22.9 µs | 22.9 µs | 12.3 µs | 40.1 µs | 21.1 µs |
| long_string_64k | **2.0 µs** | 33.0 µs | 33.1 µs | 2.1 µs | 2.2 µs | 21.8 µs | 49.6 µs | 4.2 µs |
| long_string_1m | **34.5 µs** | 526 µs | 527 µs | 37.0 µs | 37.0 µs | 399 µs | 793 µs | 76.9 µs |
| escaped_strings | 19.3 µs | 126 µs | 127 µs | 9.7 µs | **9.7 µs** | 77.3 µs | 418 µs | 98.8 µs |
| ints_10k | **60.8 µs** | 153 µs | 153 µs | 214 µs | 214 µs | 81.6 µs | 202 µs | 137 µs |
| floats_10k | **84.5 µs** | 223 µs | 222 µs | 195 µs | 195 µs | 179 µs | 276 µs | 203 µs |
| twitter_100 | **57.5 µs** | 138 µs | 205 µs | 117 µs | 117 µs | 64.4 µs | 313 µs | 149 µs |
| tree_3280 | 162 µs | 365 µs | 535 µs | 386 µs | 387 µs | **160 µs** | 705 µs | 288 µs |
| deep_chain_200 | **9.1 µs** | 17.3 µs | 26.8 µs | 24.4 µs | 24.4 µs | 10.0 µs | 39.5 µs | 14.6 µs |

### Allocations

- **Valid:** never allocates.
- **Marshal:** makes one allocation, the returned slice. `MarshalAppend` into a slice with enough capacity makes none.
- **Unmarshal:** typed decoding of the `small` and `medium` records makes 1–2 allocations per call.

## Testing

```sh
go test ./...
go test -tags purego ./...
go test -race ./...
```

Beyond the unit tests, the suite includes:

- **Differential tests:** results compared with `encoding/json`.
- **Fuzz targets:** `go test -run '^$' -fuzz FuzzValidDifferential .`, plus the others.
- **Guard-page tests (Linux):** check that no input is ever read past its end.
- **Conformance:** checks against [JSONTestSuite](https://github.com/nst/JSONTestSuite). Point `JSONTESTSUITE_DIR` at its `test_parsing` folder and run `go test -run TestJSONTestSuite .`.
- **Scaling test:** confirms adversarial inputs take linear time. Run it with `ALOS_SCALING=1 go test -run TestAdversarialInputsScaleLinearly .`.

## License

MIT, see [LICENSE](LICENSE). Part of the float parser is adapted from the Go standard library; its BSD license notice is in [THIRD_PARTY_NOTICES](THIRD_PARTY_NOTICES).
