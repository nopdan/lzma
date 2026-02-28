package lzma

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const LZMAHeaderSize = 13

type Properties struct {
	LC int
	LP int
	PB int
}

type Header struct {
	Properties       Properties
	DictionarySize   uint32
	UncompressedSize int64
}

var (
	ErrInvalidProperties = errors.New("invalid lzma properties")
	ErrInvalidHeader     = errors.New("invalid lzma header")
)

func ParseProperties(b byte) (Properties, error) {
	if b >= 225 {
		return Properties{}, ErrInvalidProperties
	}

	lc := int(b % 9)
	rest := int(b / 9)
	lp := rest % 5
	pb := rest / 5

	if lc > 8 || lp > 4 || pb > 4 {
		return Properties{}, ErrInvalidProperties
	}

	return Properties{LC: lc, LP: lp, PB: pb}, nil
}

func ParseLZMAHeader(raw [LZMAHeaderSize]byte) (Header, error) {
	props, err := ParseProperties(raw[0])
	if err != nil {
		return Header{}, err
	}

	dictSize := binary.LittleEndian.Uint32(raw[1:5])
	if dictSize == 0 {
		dictSize = 1
	}

	sizeRaw := binary.LittleEndian.Uint64(raw[5:13])
	uncompressedSize := int64(sizeRaw)
	if sizeRaw == ^uint64(0) {
		uncompressedSize = -1
	}

	return Header{
		Properties:       props,
		DictionarySize:   dictSize,
		UncompressedSize: uncompressedSize,
	}, nil
}

func ReadLZMAHeader(r io.Reader) (Header, error) {
	var raw [LZMAHeaderSize]byte
	if _, err := io.ReadFull(r, raw[:]); err != nil {
		if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
			return Header{}, fmt.Errorf("%w: short header", ErrInvalidHeader)
		}
		return Header{}, err
	}
	return ParseLZMAHeader(raw)
}
