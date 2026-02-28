package lzma

import (
	"bytes"
	"testing"
)

func TestNewRangeDecoderRejectsInvalidMarker(t *testing.T) {
	_, err := NewRangeDecoder(bytes.NewReader([]byte{0x01, 0, 0, 0, 0}))
	if err == nil {
		t.Fatalf("expected invalid marker error")
	}
}

func TestDecodeBitUpdatesProbability(t *testing.T) {
	payload := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	rd, err := NewRangeDecoder(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("NewRangeDecoder returned error: %v", err)
	}

	prob := uint16(1024)
	_, err = rd.DecodeBit(&prob)
	if err != nil {
		t.Fatalf("DecodeBit returned error: %v", err)
	}

	if prob == 1024 {
		t.Fatalf("expected probability to change")
	}
}
