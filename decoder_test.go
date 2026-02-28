package lzma

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"
)

func TestDecodeLZMALiteralOnlyZeros(t *testing.T) {
	var stream bytes.Buffer

	var hdr [LZMAHeaderSize]byte
	hdr[0] = 0x5d
	binary.LittleEndian.PutUint32(hdr[1:5], 1<<20)
	binary.LittleEndian.PutUint64(hdr[5:13], 4)
	stream.Write(hdr[:])
	stream.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00})
	stream.Write(make([]byte, 64))

	dec := NewDecoder()
	var out bytes.Buffer
	if err := dec.DecodeLZMA(bytes.NewReader(stream.Bytes()), &out); err != nil {
		t.Fatalf("DecodeLZMA returned error: %v", err)
	}

	got := out.Bytes()
	if len(got) != 4 {
		t.Fatalf("expected 4 output bytes, got %d", len(got))
	}
	for i, b := range got {
		if b != 0 {
			t.Fatalf("expected output[%d] to be 0, got %d", i, b)
		}
	}
}

func TestDecodeLZMAZeroSizeReturnsNil(t *testing.T) {
	var stream bytes.Buffer

	var hdr [LZMAHeaderSize]byte
	hdr[0] = 0x5d
	binary.LittleEndian.PutUint32(hdr[1:5], 4096)
	binary.LittleEndian.PutUint64(hdr[5:13], 0)
	stream.Write(hdr[:])
	stream.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00})

	dec := NewDecoder()
	var out bytes.Buffer
	if err := dec.DecodeLZMA(bytes.NewReader(stream.Bytes()), &out); err != nil {
		t.Fatalf("expected nil error, got: %v", err)
	}
}

func TestDecodeLZMAUnknownSizeNotSupported(t *testing.T) {
	var stream bytes.Buffer

	var hdr [LZMAHeaderSize]byte
	hdr[0] = 0x5d
	binary.LittleEndian.PutUint32(hdr[1:5], 4096)
	binary.LittleEndian.PutUint64(hdr[5:13], ^uint64(0))
	stream.Write(hdr[:])
	stream.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00})

	dec := NewDecoder()
	var out bytes.Buffer
	err := dec.DecodeLZMA(bytes.NewReader(stream.Bytes()), &out)
	if err == nil {
		t.Fatalf("expected decode error for truncated unknown-size stream")
	}
	if errors.Is(err, ErrNotImplemented) {
		t.Fatalf("unknown-size path should not return ErrNotImplemented: %v", err)
	}
}
