package lzma

import (
	"errors"
	"fmt"
	"io"
)

// Range coder constants used by LZMA bit decoding.
const (
	rangeTopValue       = uint32(1 << 24)
	bitModelTotal       = uint32(1 << 11)
	moveBits            = uint32(5)
	rangeDecoderBufSize = 64 << 10
)

// rangeDecoder decodes entropy-coded bits from the LZMA range coder stream.
type rangeDecoder struct {
	r    io.Reader
	buf  [rangeDecoderBufSize]byte
	n    int
	off  int
	code uint32
	rng  uint32
}

// ErrInvalidRangeCoderHeader reports invalid first byte in range coder init sequence.
var ErrInvalidRangeCoderHeader = errors.New("invalid range coder header")

// newRangeDecoder constructs a range decoder and consumes the 5-byte coder header.
func newRangeDecoder(r io.Reader) (*rangeDecoder, error) {
	rd := &rangeDecoder{}
	if err := rd.reset(r); err != nil {
		return nil, err
	}
	return rd, nil
}

// reset reinitializes the decoder for a new stream and consumes the 5-byte coder header.
func (rd *rangeDecoder) reset(r io.Reader) error {
	var init [5]byte
	if _, err := io.ReadFull(r, init[:]); err != nil {
		return err
	}
	if init[0] != 0x00 {
		return fmt.Errorf("%w: first byte is 0x%02x", ErrInvalidRangeCoderHeader, init[0])
	}

	rd.r = r
	rd.n = 0
	rd.off = 0
	rd.rng = ^uint32(0)
	rd.code = uint32(init[1])<<24 | uint32(init[2])<<16 | uint32(init[3])<<8 | uint32(init[4])

	if err := rd.fillBuf(); err != nil {
		return err
	}
	return nil
}

// release clears reader references so pooled decoders don't retain stream sources.
func (rd *rangeDecoder) release() {
	rd.r = nil
	rd.n = 0
	rd.off = 0
}

// fillBuf refills the internal read buffer from the underlying reader.
func (rd *rangeDecoder) fillBuf() error {
	n, err := rd.r.Read(rd.buf[:])
	if n > 0 {
		rd.off = 0
		rd.n = n
		return nil
	}
	return err
}

// readByte returns one byte from the buffered stream.
func (rd *rangeDecoder) readByte() (byte, error) {
	if rd.off >= rd.n {
		if err := rd.fillBuf(); err != nil {
			return 0, err
		}
		if rd.n == 0 {
			return 0, io.ErrUnexpectedEOF
		}
	}
	b := rd.buf[rd.off]
	rd.off++
	return b, nil
}

// normalize keeps rng above top threshold by shifting in source bytes.
func (rd *rangeDecoder) normalize() error {
	for rd.rng < rangeTopValue {
		b, err := rd.readByte()
		if err != nil {
			return err
		}
		rd.code = (rd.code << 8) | uint32(b)
		rd.rng <<= 8
	}
	return nil
}

// decodeBit decodes one bit using the adaptive probability model.
func (rd *rangeDecoder) decodeBit(prob *uint16) (uint32, error) {
	rng := rd.rng
	code := rd.code

	bound := (rng >> 11) * uint32(*prob)
	if code < bound {
		rng = bound
		// Bound branch decodes bit=0 and shifts probability toward 1.
		*prob = uint16(uint32(*prob) + ((bitModelTotal - uint32(*prob)) >> moveBits))
		for rng < rangeTopValue {
			b, err := rd.readByte()
			if err != nil {
				return 0, err
			}
			code = (code << 8) | uint32(b)
			rng <<= 8
		}
		rd.code = code
		rd.rng = rng
		return 0, nil
	}

	rng -= bound
	code -= bound
	// Else branch decodes bit=1 and shifts probability toward 0.
	*prob = uint16(uint32(*prob) - (uint32(*prob) >> moveBits))
	for rng < rangeTopValue {
		b, err := rd.readByte()
		if err != nil {
			return 0, err
		}
		code = (code << 8) | uint32(b)
		rng <<= 8
	}
	rd.code = code
	rd.rng = rng
	return 1, nil
}

// decodeDirectBits decodes raw direct bits without probability models.
func (rd *rangeDecoder) decodeDirectBits(numTotalBits uint32) (uint32, error) {
	var result uint32
	for range numTotalBits {
		rd.rng >>= 1
		t := (rd.code - rd.rng) >> 31
		rd.code -= rd.rng & (t - 1)
		result = (result << 1) | (1 - t)
		if rd.rng < rangeTopValue {
			b, err := rd.readByte()
			if err != nil {
				return 0, err
			}
			rd.code = (rd.code << 8) | uint32(b)
			rd.rng <<= 8
		}
	}
	return result, nil
}
