package lzma

import (
	"bytes"
	"encoding/binary"
	"testing"

	reflzma "github.com/ulikunitz/xz/lzma"
)

func compressWithReferenceLZMA(tb testing.TB, src []byte) []byte {
	tb.Helper()

	var buf bytes.Buffer
	w, err := reflzma.NewWriter(&buf)
	if err != nil {
		tb.Fatalf("reference lzma.NewWriter failed: %v", err)
	}
	if _, err := w.Write(src); err != nil {
		tb.Fatalf("reference writer write failed: %v", err)
	}
	if err := w.Close(); err != nil {
		tb.Fatalf("reference writer close failed: %v", err)
	}
	return buf.Bytes()
}

func TestDecodeLZMARoundTripUnknownSize(t *testing.T) {
	src := bytes.Repeat([]byte("abcabcXYZ123"), 64)
	compressed := compressWithReferenceLZMA(t, src)

	if len(compressed) < LZMAHeaderSize+5 {
		t.Fatalf("compressed stream too short: %d", len(compressed))
	}
	binary.LittleEndian.PutUint64(compressed[5:13], ^uint64(0))

	dec := NewDecoder()
	var out bytes.Buffer
	if err := dec.DecodeLZMA(bytes.NewReader(compressed), &out); err != nil {
		t.Fatalf("DecodeLZMA unknown-size failed: %v", err)
	}
	if !bytes.Equal(out.Bytes(), src) {
		t.Fatalf("unknown-size decode mismatch: got %d bytes, want %d", out.Len(), len(src))
	}
}

func TestDecodeLZMARoundTripKnownSize(t *testing.T) {
	src := bytes.Repeat([]byte("The quick brown fox jumps over the lazy dog."), 48)
	compressed := compressWithReferenceLZMA(t, src)

	if len(compressed) < LZMAHeaderSize+5 {
		t.Fatalf("compressed stream too short: %d", len(compressed))
	}
	binary.LittleEndian.PutUint64(compressed[5:13], uint64(len(src)))

	dec := NewDecoder()
	var out bytes.Buffer
	if err := dec.DecodeLZMA(bytes.NewReader(compressed), &out); err != nil {
		t.Fatalf("DecodeLZMA known-size failed: %v", err)
	}
	if !bytes.Equal(out.Bytes(), src) {
		t.Fatalf("known-size decode mismatch: got %d bytes, want %d", out.Len(), len(src))
	}
}
