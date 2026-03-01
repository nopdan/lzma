package lzma

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

// lzmaHeaderSize is the size in bytes of the legacy .lzma stream header.
const lzmaHeaderSize = 13

// properties stores lc/lp/pb fields encoded in the first header byte.
type properties struct {
	LC uint8
	LP uint8
	PB uint8
}

// header stores parsed .lzma header metadata required by the decoder.
type header struct {
	Properties          properties
	DictionarySize      uint32
	UncompressedSize    uint64
	HasUncompressedSize bool
}

var (
	// ErrInvalidProperties reports malformed lc/lp/pb combinations.
	ErrInvalidProperties = errors.New("invalid lzma properties")
	// ErrInvalidHeader reports malformed or truncated header bytes.
	ErrInvalidHeader = errors.New("invalid lzma header")
)

// parseProperties decodes lc/lp/pb values from the packed properties byte.
func parseProperties(b byte) (properties, error) {
	if b >= 225 {
		return properties{}, ErrInvalidProperties
	}

	lc := uint8(b % 9)
	rest := b / 9
	lp := uint8(rest % 5)
	pb := uint8(rest / 5)

	if lc > 8 || lp > 4 || pb > 4 {
		return properties{}, ErrInvalidProperties
	}

	return properties{LC: lc, LP: lp, PB: pb}, nil
}

// parseLZMAHeader parses a 13-byte .lzma header into structured metadata.
func parseLZMAHeader(raw [lzmaHeaderSize]byte) (header, error) {
	props, err := parseProperties(raw[0])
	if err != nil {
		return header{}, err
	}

	dictSize := binary.LittleEndian.Uint32(raw[1:5])
	if dictSize == 0 {
		dictSize = 1
	}

	sizeRaw := binary.LittleEndian.Uint64(raw[5:13])
	hasUncompressedSize := sizeRaw != ^uint64(0)
	if !hasUncompressedSize {
		sizeRaw = 0
	}

	return header{
		Properties:          props,
		DictionarySize:      dictSize,
		UncompressedSize:    sizeRaw,
		HasUncompressedSize: hasUncompressedSize,
	}, nil
}

// readLZMAHeader reads and parses a full .lzma header from r.
func readLZMAHeader(r io.Reader) (header, error) {
	var raw [lzmaHeaderSize]byte
	if _, err := io.ReadFull(r, raw[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return header{}, fmt.Errorf("%w: short header", ErrInvalidHeader)
		}
		return header{}, err
	}
	return parseLZMAHeader(raw)
}
