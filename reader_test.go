package lzma

import (
	"bytes"
	"encoding/binary"
	"io"
	"testing"
)

func TestNewReaderZeroSizeImmediateEOF(t *testing.T) {
	var stream bytes.Buffer
	var hdr [HeaderSize]byte
	hdr[0] = 0x5d
	binary.LittleEndian.PutUint32(hdr[1:5], 1<<20)
	binary.LittleEndian.PutUint64(hdr[5:13], 0)
	stream.Write(hdr[:])
	stream.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00})

	r, err := NewReader(bytes.NewReader(stream.Bytes()))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}
	defer r.Close()

	buf := make([]byte, 8)
	n, err := r.Read(buf)
	if n != 0 || err != io.EOF {
		t.Fatalf("Read() = (%d, %v), want (0, io.EOF)", n, err)
	}
}

func TestReaderCloseIdempotent(t *testing.T) {
	var stream bytes.Buffer
	var hdr [HeaderSize]byte
	hdr[0] = 0x5d
	binary.LittleEndian.PutUint32(hdr[1:5], 1<<20)
	binary.LittleEndian.PutUint64(hdr[5:13], 0)
	stream.Write(hdr[:])
	stream.Write([]byte{0x00, 0x00, 0x00, 0x00, 0x00})

	r, err := NewReader(bytes.NewReader(stream.Bytes()))
	if err != nil {
		t.Fatalf("NewReader failed: %v", err)
	}

	if err := r.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("second Close failed: %v", err)
	}
}
