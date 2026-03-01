# lzma

A Go `.lzma` raw-stream decompressor with a standard `io.Reader` API.

## Scope

- Supports `.lzma` header parsing (`lc/lp/pb`, dictionary size, known/unknown uncompressed size).
- Supports core decode paths: literal / match / rep / short-rep.
- Supports both stream ending modes:
  - known size from header (`UncompressedSize`)
  - unknown size via end marker in stream

## Not Supported

- `.xz` container format
- LZMA2

## Quick Start

```go
package main

import (
    "io"
    "os"

    "github.com/nopdan/lzma"
)

func main() {
    in, err := os.Open("data.lzma")
    if err != nil {
        panic(err)
    }
    defer in.Close()

    r, err := lzma.NewReader(in)
    if err != nil {
        panic(err)
    }
    defer r.Close()

    out, err := os.Create("data.out")
    if err != nil {
        panic(err)
    }
    defer out.Close()

    if _, err := io.Copy(out, r); err != nil {
        panic(err)
    }
}
```

## Public API

- `NewReader(r io.Reader) (*Reader, error)`
- `(*Reader).Read(p []byte) (int, error)`
- `(*Reader).Close() error`

## Exported Errors

- `ErrInvalidHeader`
- `ErrInvalidProperties`
- `ErrInvalidRangeCoderHeader`
- `ErrNotImplemented`
- `ErrDataAfterEndMarker`
- `ErrOutputOverrun`

## Performance Notes

- Decoder instances are reused through `sync.Pool`.
- Range-decoder state is reset and reused across streams.
- Dictionary window uses grow-only capacity behavior.
- Probability model slices are reused by capacity in `initModels`.
- Hot path writes directly in pull mode to reduce extra copying.

## Dependencies

Runtime implementation does not rely on external compression libraries.

External modules listed in `go.mod` are used only in `_test.go` files for:

- correctness cross-checks against reference implementations
- benchmark comparisons across implementations

In this repository, these test/benchmark-only dependencies are:

- `github.com/ulikunitz/xz`
- `github.com/itchio/lzma`

## Caveats

- Input must be raw `.lzma` stream, not `.xz`.
- Call `Close()` to release pooled resources early.
- Benchmark input file is configured by `benchmarkFilePath` in `decoder_benchmark_test.go`.
  Default: `333_53119_SRJ.lzma` in project root.

## Test

```bash
go test ./...
```

The test suite includes random-sample compression + SHA-256 verification for known-size and unknown-size flows.

## Benchmark Usage

Memory-read benchmark (compressed file loaded into memory first):

```bash
go test -run '^$' -bench '^BenchmarkDecodeImplementationsMemory$' -benchmem ./...
```

File-read benchmark (opens file every iteration, includes I/O overhead):

```bash
go test -run '^$' -bench '^BenchmarkDecodeImplementationsFile$' -benchmem ./...
```

Single implementation examples:

```bash
go test -run '^$' -bench '^BenchmarkDecodeImplementationsMemory/Our$' -benchmem -benchtime=5s ./...
go test -run '^$' -bench '^BenchmarkDecodeImplementationsFile/Our$' -benchmem -benchtime=5s ./...
```

## Benchmark Example Results

Environment: Windows / AMD Ryzen 7 5700X / `-benchtime=1x` / benchmark file `333_53119_SRJ.lzma`.

### Memory (`BenchmarkDecodeImplementationsMemory`)

- Our: `122.02 MB/s`, `16884520 B/op`, `27 allocs/op`
- Ulikunitz: `101.18 MB/s`, `65696560 B/op`, `3055020 allocs/op`
- Itchio: `112.39 MB/s`, `16810464 B/op`, `119 allocs/op`

### File (`BenchmarkDecodeImplementationsFile`)

- Our: `119.85 MB/s`, `16894616 B/op`, `41 allocs/op`
- Ulikunitz: `1.73 MB/s`, `65708040 B/op`, `3055061 allocs/op`
- Itchio: `109.60 MB/s`, `16821816 B/op`, `148 allocs/op`

> These numbers are from a single run and will vary by machine, thermal state, and disk/cache conditions.
