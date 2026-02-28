package lzma

import (
	"errors"
	"fmt"
	"io"
)

const (
	rangeTopValue = uint32(1 << 24)
	bitModelTotal = uint32(1 << 11)
	moveBits      = uint32(5)
)

type RangeDecoder struct {
	r    io.Reader
	code uint32
	rng  uint32
}

var ErrInvalidRangeCoderHeader = errors.New("invalid range coder header")

func NewRangeDecoder(r io.Reader) (*RangeDecoder, error) {
	var init [5]byte
	if _, err := io.ReadFull(r, init[:]); err != nil {
		return nil, err
	}
	if init[0] != 0x00 {
		return nil, fmt.Errorf("%w: first byte is 0x%02x", ErrInvalidRangeCoderHeader, init[0])
	}

	rd := &RangeDecoder{r: r, rng: ^uint32(0)}
	rd.code = uint32(init[1])<<24 | uint32(init[2])<<16 | uint32(init[3])<<8 | uint32(init[4])
	return rd, nil
}

func (rd *RangeDecoder) Normalize() error {
	for rd.rng < rangeTopValue {
		var b [1]byte
		if _, err := io.ReadFull(rd.r, b[:]); err != nil {
			return err
		}
		rd.code = (rd.code << 8) | uint32(b[0])
		rd.rng <<= 8
	}
	return nil
}

func (rd *RangeDecoder) DecodeBit(prob *uint16) (uint32, error) {
	bound := (rd.rng >> 11) * uint32(*prob)
	if rd.code < bound {
		rd.rng = bound
		*prob = uint16(uint32(*prob) + ((bitModelTotal - uint32(*prob)) >> moveBits))
		if err := rd.Normalize(); err != nil {
			return 0, err
		}
		return 0, nil
	}

	rd.rng -= bound
	rd.code -= bound
	*prob = uint16(uint32(*prob) - (uint32(*prob) >> moveBits))
	if err := rd.Normalize(); err != nil {
		return 0, err
	}
	return 1, nil
}

func (rd *RangeDecoder) DecodeDirectBits(numTotalBits int) (uint32, error) {
	var result uint32
	for i := 0; i < numTotalBits; i++ {
		rd.rng >>= 1
		t := (rd.code - rd.rng) >> 31
		rd.code -= rd.rng & (t - 1)
		result = (result << 1) | (1 - t)
		if rd.rng < rangeTopValue {
			var b [1]byte
			if _, err := io.ReadFull(rd.r, b[:]); err != nil {
				return 0, err
			}
			rd.code = (rd.code << 8) | uint32(b[0])
			rd.rng <<= 8
		}
	}
	return result, nil
}
