package lzma

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

func TestNewRangeDecoderRejectsInvalidMarker(t *testing.T) {
	_, err := newRangeDecoder(bytes.NewReader([]byte{0x01, 0, 0, 0, 0}))
	if !errors.Is(err, ErrInvalidRangeCoderHeader) {
		t.Fatalf("expected ErrInvalidRangeCoderHeader, got: %v", err)
	}
}

func TestNewRangeDecoderShortHeader(t *testing.T) {
	_, err := newRangeDecoder(bytes.NewReader([]byte{0x00, 0x00, 0x00}))
	if !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("expected io.ErrUnexpectedEOF, got: %v", err)
	}
}

func TestDecodeBitUpdatesProbability(t *testing.T) {
	payload := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00}
	rd, err := newRangeDecoder(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("newRangeDecoder returned error: %v", err)
	}

	prob := uint16(1024)
	_, err = rd.decodeBit(&prob)
	if err != nil {
		t.Fatalf("decodeBit returned error: %v", err)
	}

	if prob == 1024 {
		t.Fatalf("expected probability to change")
	}
}

func TestDecodeDirectBits(t *testing.T) {
	payload := []byte{0x00, 0x12, 0x34, 0x56, 0x78, 0xff, 0xee, 0xdd, 0xcc}
	rd, err := newRangeDecoder(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("newRangeDecoder returned error: %v", err)
	}

	got, err := rd.decodeDirectBits(8)
	if err != nil {
		t.Fatalf("decodeDirectBits returned error: %v", err)
	}
	if got > 0xff {
		t.Fatalf("DecodeDirectBits(8) returned invalid value: %d", got)
	}
}

func TestRangeDecoderRelease(t *testing.T) {
	payload := []byte{0x00, 0, 0, 0, 0, 0}
	rd, err := newRangeDecoder(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("newRangeDecoder returned error: %v", err)
	}
	rd.release()
	if rd.r != nil {
		t.Fatalf("release should clear reader reference")
	}
}
