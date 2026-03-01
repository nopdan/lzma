package lzma

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestDecodeLZMALiteralOnlyZeros(t *testing.T) {
	var stream bytes.Buffer

	var hdr [lzmaHeaderSize]byte
	hdr[0] = 0x5d
	binary.LittleEndian.PutUint32(hdr[1:5], 1<<20)
	binary.LittleEndian.PutUint64(hdr[5:13], 4)
	stream.Write(hdr[:])
	stream.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00})
	stream.Write(make([]byte, 64))

	r, err := NewReader(bytes.NewReader(stream.Bytes()))
	if err != nil {
		t.Fatalf("NewReader returned error: %v", err)
	}
	defer r.Close()

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read decoded output failed: %v", err)
	}

	got := out
	if len(got) != 4 {
		t.Fatalf("expected 4 output bytes, got %d", len(got))
	}
	for i, b := range got {
		if b != 0 {
			t.Fatalf("expected output[%d] to be 0, got %d", i, b)
		}
	}
}

func TestDecodeLZMAZeroSizeReturnsEOF(t *testing.T) {
	var stream bytes.Buffer

	var hdr [lzmaHeaderSize]byte
	hdr[0] = 0x5d
	binary.LittleEndian.PutUint32(hdr[1:5], 4096)
	binary.LittleEndian.PutUint64(hdr[5:13], 0)
	stream.Write(hdr[:])
	stream.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00})

	r, err := NewReader(bytes.NewReader(stream.Bytes()))
	if err != nil {
		t.Fatalf("NewReader returned error: %v", err)
	}
	defer r.Close()

	buf := make([]byte, 1)
	n, err := r.Read(buf)
	if n != 0 || !errors.Is(err, io.EOF) {
		t.Fatalf("Read() = (%d, %v), want (0, io.EOF)", n, err)
	}
}

func TestDecodeLZMAUnknownSizePath(t *testing.T) {
	var stream bytes.Buffer

	var hdr [lzmaHeaderSize]byte
	hdr[0] = 0x5d
	binary.LittleEndian.PutUint32(hdr[1:5], 4096)
	binary.LittleEndian.PutUint64(hdr[5:13], ^uint64(0))
	stream.Write(hdr[:])
	stream.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00})

	r, err := NewReader(bytes.NewReader(stream.Bytes()))
	if err != nil {
		t.Fatalf("NewReader returned error: %v", err)
	}
	defer r.Close()

	_, err = io.ReadAll(r)
	if errors.Is(err, ErrNotImplemented) {
		t.Fatalf("unknown-size path should not return ErrNotImplemented: %v", err)
	}
}

func TestDecodeLZMAInvalidPropertiesHeader(t *testing.T) {
	var raw [lzmaHeaderSize]byte
	raw[0] = 0xFF
	binary.LittleEndian.PutUint32(raw[1:5], 4096)
	binary.LittleEndian.PutUint64(raw[5:13], 10)

	_, err := NewReader(bytes.NewReader(raw[:]))
	if !errors.Is(err, ErrInvalidProperties) {
		t.Fatalf("expected ErrInvalidProperties, got: %v", err)
	}
}
