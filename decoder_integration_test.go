package lzma

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func TestDecodeRandomCompressedSampleKnownSizeHash(t *testing.T) {
	src := randomBytes(t, 2026030101, 2*1024*1024)
	compressed := compressWithReferenceLZMA(t, src, true)

	r, err := NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer r.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("decode random known-size sample failed: %v", err)
	}

	if len(out) != len(src) {
		t.Fatalf("decoded size mismatch: got %d want %d", len(out), len(src))
	}
	if got, want := sha256Hex(out), sha256Hex(src); got != want {
		t.Fatalf("decoded hash mismatch: got %s want %s", got, want)
	}
}

func TestDecodeRandomCompressedSampleUnknownSizeHash(t *testing.T) {
	src := randomBytes(t, 2026030102, 1024*1024)
	compressed := compressWithReferenceLZMA(t, src, false)

	if len(compressed) < HeaderSize+5 {
		t.Fatalf("compressed stream too short: %d", len(compressed))
	}
	binary.LittleEndian.PutUint64(compressed[5:13], ^uint64(0))

	r, err := NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer r.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("decode random unknown-size sample failed: %v", err)
	}

	if len(out) != len(src) {
		t.Fatalf("decoded size mismatch: got %d want %d", len(out), len(src))
	}
	if got, want := sha256Hex(out), sha256Hex(src); got != want {
		t.Fatalf("decoded hash mismatch: got %s want %s", got, want)
	}
}
