package lzma

import (
	"bytes"
	"testing"
)

func TestDecompressRoundTrip(t *testing.T) {
	src := bytes.Repeat([]byte("lzma-api-test-"), 80)
	compressed := compressWithReferenceLZMA(t, src)

	out, err := Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}
	if !bytes.Equal(out, src) {
		t.Fatalf("Decompress mismatch: got %d bytes, want %d", len(out), len(src))
	}
}
