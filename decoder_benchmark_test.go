package lzma

import (
	"bytes"
	"io"
	"os"
	"testing"

	itchiolzma "github.com/itchio/lzma"
	reflzma "github.com/ulikunitz/xz/lzma"
)

const benchmarkFilePath = "333_53119_SRJ.lzma"

// const benchmarkFilePath = "silesia.tar.lzma"

func loadBenchmarkCompressedFile(b testing.TB) []byte {
	b.Helper()
	data, err := os.ReadFile(benchmarkFilePath)
	if err != nil {
		b.Fatalf("read benchmark file failed: %v", err)
	}
	return data
}

func detectDecodedSize(b testing.TB, compressed []byte) int64 {
	b.Helper()
	r, err := reflzma.NewReader(bytes.NewReader(compressed))
	if err != nil {
		b.Fatalf("ulikunitz NewReader for size probe failed: %v", err)
	}
	n, err := io.Copy(io.Discard, r)
	if err != nil {
		b.Fatalf("ulikunitz decode size probe failed: %v", err)
	}
	return n
}

func benchmarkDecodeInMemory(b *testing.B, compressed []byte, decodedSize int64, open func(io.Reader) (io.ReadCloser, error)) {
	b.Helper()
	b.SetBytes(decodedSize)
	b.ReportAllocs()
	b.ResetTimer()

	var src bytes.Reader
	for i := 0; i < b.N; i++ {
		src.Reset(compressed)
		r, err := open(&src)
		if err != nil {
			b.Fatalf("open reader failed: %v", err)
		}
		_, err = io.Copy(io.Discard, r)
		_ = r.Close()
		if err != nil {
			b.Fatalf("decode failed: %v", err)
		}
	}
}

func benchmarkDecodeFromFile(b *testing.B, filePath string, decodedSize int64, open func(io.Reader) (io.ReadCloser, error)) {
	b.Helper()
	b.SetBytes(decodedSize)
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		f, err := os.Open(filePath)
		if err != nil {
			b.Fatalf("open compressed file failed: %v", err)
		}

		r, err := open(f)
		if err != nil {
			_ = f.Close()
			b.Fatalf("open reader failed: %v", err)
		}

		_, err = io.Copy(io.Discard, r)
		_ = r.Close()
		_ = f.Close()
		if err != nil {
			b.Fatalf("decode failed: %v", err)
		}
	}
}

func openOurBenchmark(src io.Reader) (io.ReadCloser, error) {
	return NewReader(src)
}

func openUlikunitzBenchmark(src io.Reader) (io.ReadCloser, error) {
	r, err := reflzma.NewReader(src)
	if err != nil {
		return nil, err
	}
	return io.NopCloser(r), nil
}

func openItchioBenchmark(src io.Reader) (io.ReadCloser, error) {
	return itchiolzma.NewReader(src), nil
}

func BenchmarkDecodeImplementationsMemory(b *testing.B) {
	compressed := loadBenchmarkCompressedFile(b)
	decodedSize := detectDecodedSize(b, compressed)

	implementations := []struct {
		name string
		open func(io.Reader) (io.ReadCloser, error)
	}{
		{name: "Our", open: openOurBenchmark},
		{name: "Ulikunitz", open: openUlikunitzBenchmark},
		{name: "Itchio", open: openItchioBenchmark},
	}

	for _, impl := range implementations {
		impl := impl
		b.Run(impl.name, func(b *testing.B) {
			benchmarkDecodeInMemory(b, compressed, decodedSize, impl.open)
		})
	}
}

func BenchmarkDecodeImplementationsFile(b *testing.B) {
	compressed := loadBenchmarkCompressedFile(b)
	decodedSize := detectDecodedSize(b, compressed)

	implementations := []struct {
		name string
		open func(io.Reader) (io.ReadCloser, error)
	}{
		{name: "Our", open: openOurBenchmark},
		{name: "Ulikunitz", open: openUlikunitzBenchmark},
		{name: "Itchio", open: openItchioBenchmark},
	}

	for _, impl := range implementations {
		impl := impl
		b.Run(impl.name, func(b *testing.B) {
			benchmarkDecodeFromFile(b, benchmarkFilePath, decodedSize, impl.open)
		})
	}
}
