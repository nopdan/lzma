package lzma

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/rand"
	"testing"

	reflzma "github.com/ulikunitz/xz/lzma"
)

func randomBytes(tb testing.TB, seed int64, size int) []byte {
	tb.Helper()
	rng := rand.New(rand.NewSource(seed))
	buf := make([]byte, size)
	if _, err := rng.Read(buf); err != nil {
		tb.Fatalf("generate random bytes failed: %v", err)
	}
	return buf
}

func compressWithReferenceLZMA(tb testing.TB, src []byte, sizeInHeader bool) []byte {
	tb.Helper()

	var out bytes.Buffer
	cfg := reflzma.WriterConfig{SizeInHeader: sizeInHeader, Size: int64(len(src))}
	w, err := cfg.NewWriter(&out)
	if err != nil {
		tb.Fatalf("reference NewWriter failed: %v", err)
	}
	if _, err := w.Write(src); err != nil {
		tb.Fatalf("reference writer write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		tb.Fatalf("reference writer close failed: %v", err)
	}
	return out.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
