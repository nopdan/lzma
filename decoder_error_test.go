package lzma

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestDecodeLZMARejectsInvalidRangeHeader(t *testing.T) {
	var stream bytes.Buffer
	var hdr [HeaderSize]byte
	hdr[0] = 0x5d
	binary.LittleEndian.PutUint32(hdr[1:5], 1<<20)
	binary.LittleEndian.PutUint64(hdr[5:13], 1)
	stream.Write(hdr[:])
	stream.Write([]byte{0x01, 0x00, 0x00, 0x00, 0x00})

	r, err := NewReader(bytes.NewReader(stream.Bytes()))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer r.Close()

	_, err = io.ReadAll(r)
	if !errors.Is(err, ErrInvalidRangeCoderHeader) {
		t.Fatalf("expected ErrInvalidRangeCoderHeader, got: %v", err)
	}
}

func TestNewReaderRejectsShortHeader(t *testing.T) {
	_, err := NewReader(bytes.NewReader([]byte{0x5d, 0x00, 0x00}))
	if !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("expected ErrInvalidHeader, got: %v", err)
	}
}
