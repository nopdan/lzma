package lzma

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestParseProperties(t *testing.T) {
	props, err := ParseProperties(0)
	if err != nil {
		t.Fatalf("ParseProperties(0) returned error: %v", err)
	}
	if props.LC != 0 || props.LP != 0 || props.PB != 0 {
		t.Fatalf("unexpected properties: %+v", props)
	}

	props, err = ParseProperties(0x5d)
	if err != nil {
		t.Fatalf("ParseProperties(0x5d) returned error: %v", err)
	}
	if props.LC != 3 || props.LP != 0 || props.PB != 2 {
		t.Fatalf("unexpected properties for 0x5d: %+v", props)
	}

	if _, err := ParseProperties(225); err == nil {
		t.Fatalf("expected ParseProperties(225) to fail")
	}
}

func TestReadLZMAHeader(t *testing.T) {
	var raw [LZMAHeaderSize]byte
	raw[0] = 0x5d
	binary.LittleEndian.PutUint32(raw[1:5], 1<<20)
	binary.LittleEndian.PutUint64(raw[5:13], 123)

	hdr, err := ReadLZMAHeader(bytes.NewReader(raw[:]))
	if err != nil {
		t.Fatalf("ReadLZMAHeader returned error: %v", err)
	}
	if hdr.Properties.LC != 3 || hdr.Properties.LP != 0 || hdr.Properties.PB != 2 {
		t.Fatalf("unexpected properties: %+v", hdr.Properties)
	}
	if hdr.DictionarySize != 1<<20 {
		t.Fatalf("unexpected dictionary size: %d", hdr.DictionarySize)
	}
	if hdr.UncompressedSize != 123 {
		t.Fatalf("unexpected uncompressed size: %d", hdr.UncompressedSize)
	}
}

func TestReadLZMAHeaderUnknownSize(t *testing.T) {
	var raw [LZMAHeaderSize]byte
	raw[0] = 0x5d
	binary.LittleEndian.PutUint32(raw[1:5], 4096)
	binary.LittleEndian.PutUint64(raw[5:13], ^uint64(0))

	hdr, err := ReadLZMAHeader(bytes.NewReader(raw[:]))
	if err != nil {
		t.Fatalf("ReadLZMAHeader returned error: %v", err)
	}
	if hdr.UncompressedSize != -1 {
		t.Fatalf("expected unknown size (-1), got: %d", hdr.UncompressedSize)
	}
}
