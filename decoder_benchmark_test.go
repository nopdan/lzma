package lzma

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"

	reflzma "github.com/ulikunitz/xz/lzma"
)

func benchmarkPayload() []byte {
	return bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog. abcabcXYZ1234567890\n"), 1024)
}

func BenchmarkDecompressKnownSize(b *testing.B) {
	src := benchmarkPayload()
	compressed := compressWithReferenceLZMA(b, src)
	if len(compressed) < LZMAHeaderSize+5 {
		b.Fatalf("compressed stream too short: %d", len(compressed))
	}
	binary.LittleEndian.PutUint64(compressed[5:13], uint64(len(src)))

	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out, err := Decompress(compressed)
		if err != nil {
			b.Fatalf("Decompress failed: %v", err)
		}
		if len(out) != len(src) {
			b.Fatalf("output size mismatch: got %d want %d", len(out), len(src))
		}
	}
}

func BenchmarkDecompressUnknownSize(b *testing.B) {
	src := benchmarkPayload()
	compressed := compressWithReferenceLZMA(b, src)
	if len(compressed) < LZMAHeaderSize+5 {
		b.Fatalf("compressed stream too short: %d", len(compressed))
	}
	binary.LittleEndian.PutUint64(compressed[5:13], ^uint64(0))

	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		out, err := Decompress(compressed)
		if err != nil {
			b.Fatalf("Decompress failed: %v", err)
		}
		if len(out) != len(src) {
			b.Fatalf("output size mismatch: got %d want %d", len(out), len(src))
		}
	}
}

func BenchmarkDecoderStreamKnownSize(b *testing.B) {
	src := benchmarkPayload()
	compressed := compressWithReferenceLZMA(b, src)
	if len(compressed) < LZMAHeaderSize+5 {
		b.Fatalf("compressed stream too short: %d", len(compressed))
	}
	binary.LittleEndian.PutUint64(compressed[5:13], uint64(len(src)))

	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		dec := NewDecoder()
		var out bytes.Buffer
		if err := dec.DecodeLZMA(bytes.NewReader(compressed), &out); err != nil {
			b.Fatalf("DecodeLZMA failed: %v", err)
		}
		if out.Len() != len(src) {
			b.Fatalf("output size mismatch: got %d want %d", out.Len(), len(src))
		}
	}
}

func BenchmarkUlikunitzDecodeKnownSize(b *testing.B) {
	src := benchmarkPayload()
	compressed := compressWithReferenceLZMA(b, src)
	if len(compressed) < LZMAHeaderSize+5 {
		b.Fatalf("compressed stream too short: %d", len(compressed))
	}
	binary.LittleEndian.PutUint64(compressed[5:13], uint64(len(src)))

	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r, err := reflzma.NewReader(bytes.NewReader(compressed))
		if err != nil {
			b.Fatalf("ulikunitz NewReader failed: %v", err)
		}
		n, err := io.Copy(io.Discard, r)
		if err != nil {
			b.Fatalf("ulikunitz decode copy failed: %v", err)
		}
		if int(n) != len(src) {
			b.Fatalf("output size mismatch: got %d want %d", n, len(src))
		}
	}
}

func BenchmarkUlikunitzDecodeUnknownSize(b *testing.B) {
	src := benchmarkPayload()
	compressed := compressWithReferenceLZMA(b, src)
	if len(compressed) < LZMAHeaderSize+5 {
		b.Fatalf("compressed stream too short: %d", len(compressed))
	}
	binary.LittleEndian.PutUint64(compressed[5:13], ^uint64(0))

	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r, err := reflzma.NewReader(bytes.NewReader(compressed))
		if err != nil {
			b.Fatalf("ulikunitz NewReader failed: %v", err)
		}
		n, err := io.Copy(io.Discard, r)
		if err != nil {
			b.Fatalf("ulikunitz decode copy failed: %v", err)
		}
		if int(n) != len(src) {
			b.Fatalf("output size mismatch: got %d want %d", n, len(src))
		}
	}
}
