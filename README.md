# alos-json

[![Go Reference](https://pkg.go.dev/badge/github.com/guno1928/alos-json.svg)](https://pkg.go.dev/github.com/guno1928/alos-json)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A fast JSON library for Go. The API follows `encoding/json` (`Marshal`, `Unmarshal`, `Valid`, …), and the defaults and options follow `encoding/json/v2`. On amd64 the hot loops use SSE2 and AVX2 assembly, with AVX2 picked at run time. Every other platform gets the same results from pure Go.

```sh
go get github.com/guno1928/alos-json
```

Requires Go 1.24 or newer. The package name is `json`.

## At a glance

Geometric mean over the datasets in [`bench/`](bench), measured on an AMD Ryzen 7 5700X with Go 1.26.2. Each number is how many times longer the other library takes than alos, so **above 1.00 means alos is faster** and below 1.00 means the other library is faster.

| Operation | encoding/json | json/v2 | sonic | sonic (std) | goccy | jsoniter | segmentio | alos fastest on |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| Unmarshal into structs | 8.59 | 3.48 | 1.07 | 1.38 | 1.11 | 2.35 | 1.82 | 6 / 13 |
| Marshal structs | 2.42 | 3.13 | 0.79 | 0.99 | 0.91 | 2.24 | 1.47 | 1 / 13 |
| Unmarshal into `any` | 3.15 | 2.37 | 1.09 | 1.30 | 1.17 | 2.54 | 3.53 | 4 / 7 |
| Marshal `any` | 3.69 | 2.48 | 1.26 | 1.54 | 1.35 | 2.33 | 2.19 | 5 / 7 |
| Valid | 9.20 | 3.86 | 1.34 | 1.33 | 1.77 | 6.40 | 2.35 | 10 / 13 |

alos is the fastest overall at validating, decoding and encoding `any`, and at decoding structs. sonic and goccy are faster at encoding structs. Full tables are in [Benchmarks](#benchmarks).

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

- **Machine:** AMD Ryzen 7 5700X, Windows 11, Go 1.26.2, amd64 with AVX2.
- **Runs:** each benchmark ran for 150 ms; the tables show the best of 3 runs, or 6 runs for Valid.
- **json/v2:** measured with `GOEXPERIMENT=jsonv2`.
- **Library versions:** sonic v1.15.5 (`sonic` is the default config, `sonic (std)` is `sonic.ConfigStd`), goccy/go-json v0.12.0, json-iterator/go v1.1.12, segmentio/encoding v0.5.4.
- **Recency:** the Valid table comes from a later run on the same machine than the other tables. The Marshal tables were measured before a final change that makes float formatting match `encoding/json`; an A/B of that change on another machine measured 1–2% on float-heavy Marshal rows.

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
| small | 170 ns | 926 ns | 460 ns | 196 ns | 239 ns | **138 ns** | 220 ns | 314 ns |
| medium | 411 ns | 2.8 µs | 1.2 µs | 507 ns | 664 ns | **411 ns** | 647 ns | 732 ns |
| large_100 | 41.4 µs | 258 µs | 126 µs | 48.2 µs | 61.3 µs | **39.8 µs** | 61.1 µs | 58.4 µs |
| xlarge_10k | **4.07 ms** | 26.33 ms | 12.91 ms | 4.66 ms | 6.01 ms | 4.21 ms | 6.71 ms | 5.68 ms |
| short_strings | 35.7 µs | 250 µs | 107 µs | **26.7 µs** | 48.7 µs | 49.7 µs | 67.2 µs | 56.2 µs |
| long_string_64k | **7.2 µs** | 264 µs | 38.1 µs | 7.5 µs | 14.4 µs | 8.6 µs | 48.7 µs | 8.6 µs |
| long_string_1m | 117 µs | 4.26 ms | 569 µs | **117 µs** | 216 µs | 152 µs | 757 µs | 135 µs |
| escaped_strings | 79.2 µs | 469 µs | 289 µs | **58.1 µs** | 64.7 µs | 118 µs | 277 µs | 356 µs |
| ints_10k | **115 µs** | 1.05 ms | 366 µs | 132 µs | 130 µs | 129 µs | 215 µs | 135 µs |
| floats_10k | 215 µs | 1.60 ms | 860 µs | 234 µs | 230 µs | **185 µs** | 1.14 ms | 547 µs |
| twitter_100 | **165 µs** | 1.02 ms | 485 µs | 197 µs | 228 µs | 202 µs | 365 µs | 591 µs |
| tree_3280 | **400 µs** | 2.75 ms | 1.62 ms | 526 µs | 572 µs | 454 µs | 621 µs | 677 µs |
| deep_chain_200 | **27.0 µs** | 159 µs | 89.6 µs | 32.3 µs | 34.8 µs | 31.5 µs | 37.8 µs | 55.3 µs |

### Marshal structs

| dataset | alos | encoding/json | json/v2 | sonic | sonic (std) | goccy | jsoniter | segmentio |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| small | **102 ns** | 228 ns | 307 ns | 114 ns | 183 ns | 108 ns | 209 ns | 134 ns |
| medium | 229 ns | 493 ns | 700 ns | 210 ns | 295 ns | **196 ns** | 426 ns | 271 ns |
| large_100 | 19.5 µs | 40.9 µs | 61.8 µs | **14.9 µs** | 16.0 µs | 16.1 µs | 38.6 µs | 23.7 µs |
| xlarge_10k | 1.94 ms | 4.11 ms | 6.27 ms | 2.35 ms | 2.97 ms | **1.49 ms** | 3.89 ms | 2.46 ms |
| short_strings | 24.9 µs | 41.2 µs | 47.3 µs | 16.8 µs | 17.3 µs | **16.6 µs** | 29.2 µs | 22.4 µs |
| long_string_64k | 12.0 µs | 53.9 µs | 81.1 µs | **10.6 µs** | 12.9 µs | 13.4 µs | 75.5 µs | 23.4 µs |
| long_string_1m | 140 µs | 856 µs | 1.28 ms | **112 µs** | 262 µs | 172 µs | 1.17 ms | 408 µs |
| escaped_strings | 50.1 µs | 109 µs | 113 µs | **29.4 µs** | 46.2 µs | 42.8 µs | 116 µs | 132 µs |
| ints_10k | 91.0 µs | 177 µs | 164 µs | **57.6 µs** | 62.5 µs | 99.3 µs | 98.8 µs | 93.3 µs |
| floats_10k | 401 µs | 582 µs | 580 µs | **280 µs** | 285 µs | 383 µs | 531 µs | 520 µs |
| twitter_100 | 84.3 µs | 180 µs | 256 µs | **67.3 µs** | 73.9 µs | 70.4 µs | 188 µs | 113 µs |
| tree_3280 | 204 µs | 559 µs | 778 µs | **157 µs** | 165 µs | 221 µs | 484 µs | 303 µs |
| deep_chain_200 | 13.4 µs | 37.3 µs | 49.1 µs | **8.6 µs** | 8.8 µs | 9.8 µs | 28.9 µs | 22.9 µs |

### Unmarshal into `any`

| dataset | alos | encoding/json | json/v2 | sonic | sonic (std) | goccy | jsoniter | segmentio |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| small | **341 ns** | 865 ns | 709 ns | 376 ns | 475 ns | 377 ns | 557 ns | 685 ns |
| medium | **837 ns** | 2.3 µs | 1.7 µs | 998 ns | 1.3 µs | 1.0 µs | 1.8 µs | 1.8 µs |
| large_100 | **79.7 µs** | 212 µs | 166 µs | 90.9 µs | 119 µs | 103 µs | 181 µs | 207 µs |
| escaped_strings | 80.1 µs | 446 µs | 294 µs | **63.1 µs** | 67.2 µs | 128 µs | 324 µs | 627 µs |
| floats_10k | 286 µs | 1.49 ms | 1.05 ms | 401 µs | 406 µs | **278 µs** | 1.81 ms | 1.15 ms |
| twitter_100 | 442 µs | 1.08 ms | 780 µs | 445 µs | 552 µs | **437 µs** | 791 µs | 1.93 ms |
| tree_3280 | **1.00 ms** | 2.37 ms | 1.99 ms | 1.11 ms | 1.31 ms | 1.08 ms | 1.90 ms | 4.43 ms |

### Marshal `any`

| dataset | alos | encoding/json | json/v2 | sonic | sonic (std) | goccy | jsoniter | segmentio |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| small | **172 ns** | 691 ns | 1.0 µs | 296 ns | 390 ns | 255 ns | 419 ns | 343 ns |
| medium | **404 ns** | 2.0 µs | 2.0 µs | 646 ns | 876 ns | 653 ns | 976 ns | 968 ns |
| large_100 | **39.4 µs** | 205 µs | 74.3 µs | 60.7 µs | 70.4 µs | 61.9 µs | 97.6 µs | 95.3 µs |
| escaped_strings | 52.9 µs | 125 µs | 112 µs | **33.2 µs** | 48.6 µs | 45.9 µs | 129 µs | 138 µs |
| floats_10k | 425 µs | 800 µs | 605 µs | **358 µs** | 362 µs | 414 µs | 689 µs | 637 µs |
| twitter_100 | **181 µs** | 795 µs | 332 µs | 244 µs | 298 µs | 271 µs | 426 µs | 429 µs |
| tree_3280 | **484 µs** | 2.23 ms | 940 µs | 796 µs | 878 µs | 845 µs | 1.32 ms | 1.09 ms |

### Valid

| dataset | alos | encoding/json | json/v2 | sonic | sonic (std) | goccy | jsoniter | segmentio |
|---|---:|---:|---:|---:|---:|---:|---:|---:|
| small | **39 ns** | 198 ns | 167 ns | 75 ns | 73 ns | 58 ns | 172 ns | 186 ns |
| medium | **119 ns** | 578 ns | 369 ns | 181 ns | 180 ns | 127 ns | 477 ns | 299 ns |
| large_100 | **11.7 µs** | 55.3 µs | 32.0 µs | 17.3 µs | 17.1 µs | 11.8 µs | 46.8 µs | 19.5 µs |
| xlarge_10k | **1.25 ms** | 6.11 ms | 3.19 ms | 1.75 ms | 1.80 ms | 1.25 ms | 5.13 ms | 2.00 ms |
| short_strings | 9.9 µs | 58.3 µs | 17.6 µs | 12.4 µs | 12.5 µs | **9.4 µs** | 29.0 µs | 17.3 µs |
| long_string_64k | **1.1 µs** | 181 µs | 29.0 µs | 1.5 µs | 1.5 µs | 14.2 µs | 38.2 µs | 2.2 µs |
| long_string_1m | **19.0 µs** | 2.75 ms | 451 µs | 24.1 µs | 23.6 µs | 258 µs | 632 µs | 36.2 µs |
| escaped_strings | 21.4 µs | 247 µs | 90.5 µs | 7.4 µs | **7.3 µs** | 56.0 µs | 359 µs | 87.3 µs |
| ints_10k | **52.4 µs** | 265 µs | 98.9 µs | 104 µs | 106 µs | 65.6 µs | 203 µs | 117 µs |
| floats_10k | **71.5 µs** | 460 µs | 161 µs | 115 µs | 112 µs | 128 µs | 274 µs | 163 µs |
| twitter_100 | **47.4 µs** | 313 µs | 144 µs | 60.8 µs | 61.2 µs | 50.5 µs | 269 µs | 243 µs |
| tree_3280 | 133 µs | 611 µs | 366 µs | 205 µs | 204 µs | **122 µs** | 583 µs | 216 µs |
| deep_chain_200 | **7.6 µs** | 29.2 µs | 21.9 µs | 11.9 µs | 11.8 µs | 9.0 µs | 39.4 µs | 14.6 µs |

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
