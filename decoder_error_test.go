package lzma

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestDecodeLZMAKnownSizeWithEarlyEndMarker(t *testing.T) {
	src := bytes.Repeat([]byte("abc"), 32)
	compressed := compressWithReferenceLZMA(t, src)

	if len(compressed) < LZMAHeaderSize+5 {
		t.Fatalf("compressed stream too short: %d", len(compressed))
	}
	binary.LittleEndian.PutUint64(compressed[5:13], uint64(len(src)+128))

	dec := NewDecoder()
	var out bytes.Buffer
	err := dec.DecodeLZMA(bytes.NewReader(compressed), &out)
	if !errors.Is(err, ErrDataAfterEndMarker) {
		t.Fatalf("expected ErrDataAfterEndMarker, got: %v", err)
	}
}

func TestDecodeLZMARespectsKnownSizeLimit(t *testing.T) {
	src := bytes.Repeat([]byte("The quick brown fox "), 32)
	compressed := compressWithReferenceLZMA(t, src)

	if len(compressed) < LZMAHeaderSize+5 {
		t.Fatalf("compressed stream too short: %d", len(compressed))
	}
	const declaredSize = 8
	binary.LittleEndian.PutUint64(compressed[5:13], uint64(declaredSize))

	dec := NewDecoder()
	var out bytes.Buffer
	err := dec.DecodeLZMA(bytes.NewReader(compressed), &out)
	if err != nil {
		t.Fatalf("expected successful decode with truncation, got: %v", err)
	}
	if out.Len() != declaredSize {
		t.Fatalf("expected output length %d, got %d", declaredSize, out.Len())
	}
}

func TestDecodeLZMARejectsInvalidRangeHeader(t *testing.T) {
	var stream bytes.Buffer
	var hdr [LZMAHeaderSize]byte
	hdr[0] = 0x5d
	binary.LittleEndian.PutUint32(hdr[1:5], 1<<20)
	binary.LittleEndian.PutUint64(hdr[5:13], 1)
	stream.Write(hdr[:])
	stream.Write([]byte{0x01, 0x00, 0x00, 0x00, 0x00})

	dec := NewDecoder()
	var out bytes.Buffer
	err := dec.DecodeLZMA(bytes.NewReader(stream.Bytes()), &out)
	if !errors.Is(err, ErrInvalidRangeCoderHeader) {
		t.Fatalf("expected ErrInvalidRangeCoderHeader, got: %v", err)
	}
}
