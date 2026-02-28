package lzma

import (
	"errors"
	"fmt"
	"io"
)

var (
	ErrNotImplemented     = errors.New("lzma decode path is not fully implemented yet")
	ErrDataAfterEndMarker = errors.New("lzma stream ended by end marker before expected size")
	ErrOutputOverrun      = errors.New("lzma decoded output exceeds declared uncompressed size")
)

const (
	numStates          = 12
	literalSize        = 0x300
	probInit           = uint16(1024)
	numLenToPosStates  = 4
	numPosSlotBits     = 6
	startPosModelIndex = 4
	endPosModelIndex   = 14
	numFullDistances   = 1 << (endPosModelIndex / 2)
	numAlignBits       = 4
	alignTableSize     = 1 << numAlignBits
	matchMinLen        = 2
	numPosStatesMax    = 1 << 4
	outWriteChunkSize   = 32 << 10
)

type lenDecoder struct {
	choice  uint16
	choice2 uint16
	low     [numPosStatesMax][1 << 3]uint16
	mid     [numPosStatesMax][1 << 3]uint16
	high    [1 << 8]uint16
}

func (ld *lenDecoder) init() {
	ld.choice = probInit
	ld.choice2 = probInit
	for i := range ld.low {
		for j := range ld.low[i] {
			ld.low[i][j] = probInit
			ld.mid[i][j] = probInit
		}
	}
	for i := range ld.high {
		ld.high[i] = probInit
	}
}

func (ld *lenDecoder) decode(rd *RangeDecoder, posState int) (uint32, error) {
	bit, err := rd.DecodeBit(&ld.choice)
	if err != nil {
		return 0, err
	}
	if bit == 0 {
		return decodeBitTree(rd, ld.low[posState][:], 3)
	}

	bit, err = rd.DecodeBit(&ld.choice2)
	if err != nil {
		return 0, err
	}
	if bit == 0 {
		v, err := decodeBitTree(rd, ld.mid[posState][:], 3)
		if err != nil {
			return 0, err
		}
		return v + 8, nil
	}

	v, err := decodeBitTree(rd, ld.high[:], 8)
	if err != nil {
		return 0, err
	}
	return v + 16, nil
}

type outWindow struct {
	buf     []byte
	pos     uint32
	filled  uint64
	writer  io.Writer
	pending []byte
	pn      int
	total   uint64
	bufSize uint32
}

func newOutWindow(size uint32, writer io.Writer) *outWindow {
	if size == 0 {
		size = 1
	}
	return &outWindow{
		buf:     make([]byte, size),
		writer:  writer,
		pending: make([]byte, outWriteChunkSize),
		bufSize: size,
	}
}

func (ow *outWindow) putByte(b byte) error {
	ow.buf[ow.pos] = b
	ow.pos++
	if ow.pos == ow.bufSize {
		ow.pos = 0
	}
	ow.total++
	if ow.filled < uint64(ow.bufSize) {
		ow.filled++
	}
	ow.pending[ow.pn] = b
	ow.pn++
	if ow.pn == len(ow.pending) {
		return ow.flush()
	}
	return nil
}

func (ow *outWindow) getByte(distance uint32) (byte, error) {
	if uint64(distance) >= ow.filled {
		return 0, fmt.Errorf("invalid distance %d, only %d bytes available in window", distance, ow.filled)
	}

	idx := int64(ow.pos) - int64(distance) - 1
	if idx < 0 {
		idx += int64(ow.bufSize)
	}
	return ow.buf[idx], nil
}

func (ow *outWindow) flush() error {
	written := 0
	for written < ow.pn {
		n, err := ow.writer.Write(ow.pending[written:ow.pn])
		written += n
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
	}
	ow.pn = 0
	return nil
}

type Decoder struct {
	header Header
	rd     *RangeDecoder
	out    *outWindow

	isMatchProbs []uint16
	isRepProbs   []uint16
	isRepG0Probs []uint16
	isRepG1Probs []uint16
	isRepG2Probs []uint16
	isRep0Long   []uint16

	posSlotCoder [numLenToPosStates][1 << numPosSlotBits]uint16
	posDecoders  []uint16
	posAlign     [alignTableSize]uint16

	lenDecoder    lenDecoder
	repLenDecoder lenDecoder
	literalProbs  []uint16

	state int
	pos   int64
	prev  byte

	rep0 uint32
	rep1 uint32
	rep2 uint32
	rep3 uint32
}

func NewDecoder() *Decoder {
	return &Decoder{}
}

func (d *Decoder) Header() Header {
	return d.header
}

func (d *Decoder) DecodeLZMA(r io.Reader, w io.Writer) error {
	hdr, err := ReadLZMAHeader(r)
	if err != nil {
		return err
	}
	d.header = hdr

	rd, err := NewRangeDecoder(r)
	if err != nil {
		return err
	}
	d.rd = rd

	if hdr.UncompressedSize == 0 {
		return nil
	}

	if err := d.initModels(); err != nil {
		return err
	}
	d.out = newOutWindow(hdr.DictionarySize, w)

	hasKnownSize := hdr.UncompressedSize >= 0

	for {
		if hasKnownSize && d.pos >= hdr.UncompressedSize {
			return d.out.flush()
		}

		posState := d.currentPosState()
		idx := (d.state << hdr.Properties.PB) + posState
		isMatch, err := d.rd.DecodeBit(&d.isMatchProbs[idx])
		if err != nil {
			return err
		}

		if isMatch == 0 {
			b, err := d.decodeLiteral()
			if err != nil {
				return err
			}
			if err := d.out.putByte(b); err != nil {
				return err
			}

			d.state = updateStateLiteral(d.state)
			d.prev = b
			d.pos++
			continue
		}

		isRep, err := d.rd.DecodeBit(&d.isRepProbs[d.state])
		if err != nil {
			return err
		}

		var length uint32
		if isRep == 1 {
			isRepG0, err := d.rd.DecodeBit(&d.isRepG0Probs[d.state])
			if err != nil {
				return err
			}
			if isRepG0 == 0 {
				isRep0LongIdx := (d.state << d.header.Properties.PB) + posState
				isRep0Long, err := d.rd.DecodeBit(&d.isRep0Long[isRep0LongIdx])
				if err != nil {
					return err
				}
				if isRep0Long == 0 {
					d.state = updateStateShortRep(d.state)
					b, err := d.out.getByte(d.rep0)
					if err != nil {
						return err
					}
					if err := d.out.putByte(b); err != nil {
						return err
					}
					d.prev = b
					d.pos++
					continue
				}
			} else {
				var dist uint32
				isRepG1, err := d.rd.DecodeBit(&d.isRepG1Probs[d.state])
				if err != nil {
					return err
				}
				if isRepG1 == 0 {
					dist = d.rep1
				} else {
					isRepG2, err := d.rd.DecodeBit(&d.isRepG2Probs[d.state])
					if err != nil {
						return err
					}
					if isRepG2 == 0 {
						dist = d.rep2
					} else {
						dist = d.rep3
						d.rep3 = d.rep2
					}
					d.rep2 = d.rep1
				}
				d.rep1 = d.rep0
				d.rep0 = dist
			}

			repLen, err := d.repLenDecoder.decode(d.rd, posState)
			if err != nil {
				return err
			}
			length = repLen + matchMinLen
			d.state = updateStateRep(d.state)
		} else {
			d.rep3 = d.rep2
			d.rep2 = d.rep1
			d.rep1 = d.rep0

			mainLen, err := d.lenDecoder.decode(d.rd, posState)
			if err != nil {
				return err
			}
			length = mainLen + matchMinLen
			d.state = updateStateMatch(d.state)

			lenToPosState := getLenToPosState(length)
			posSlot, err := decodeBitTree(d.rd, d.posSlotCoder[lenToPosState][:], numPosSlotBits)
			if err != nil {
				return err
			}

			if posSlot < startPosModelIndex {
				d.rep0 = posSlot
			} else {
				numDirectBits := int((posSlot >> 1) - 1)
				d.rep0 = (2 | (posSlot & 1)) << uint(numDirectBits)

				if posSlot < endPosModelIndex {
					base := int(d.rep0 - posSlot - 1)
					extra, err := reverseDecodeWithOffset(d.rd, d.posDecoders, base, numDirectBits)
					if err != nil {
						return err
					}
					d.rep0 += extra
				} else {
					direct, err := d.rd.DecodeDirectBits(numDirectBits - numAlignBits)
					if err != nil {
						return err
					}
					d.rep0 += direct << numAlignBits

					align, err := reverseDecodeBitTree(d.rd, d.posAlign[:], numAlignBits)
					if err != nil {
						return err
					}
					d.rep0 += align
				}

				if d.rep0 == ^uint32(0) {
					if !hasKnownSize {
						return d.out.flush()
					}
					return ErrDataAfterEndMarker
				}
			}
		}

		if hasKnownSize && d.pos+int64(length) > hdr.UncompressedSize {
			return ErrOutputOverrun
		}

		if err := d.copyMatch(length); err != nil {
			return err
		}
	}
}

func (d *Decoder) initModels() error {
	hdr := d.header
	if hdr.Properties.LC < 0 || hdr.Properties.LC > 8 || hdr.Properties.LP < 0 || hdr.Properties.LP > 4 || hdr.Properties.PB < 0 || hdr.Properties.PB > 4 {
		return ErrInvalidProperties
	}

	posStates := 1 << hdr.Properties.PB
	d.isMatchProbs = make([]uint16, numStates*posStates)
	d.isRepProbs = make([]uint16, numStates)
	d.isRepG0Probs = make([]uint16, numStates)
	d.isRepG1Probs = make([]uint16, numStates)
	d.isRepG2Probs = make([]uint16, numStates)
	d.isRep0Long = make([]uint16, numStates*posStates)
	d.posDecoders = make([]uint16, numFullDistances-startPosModelIndex)

	literalContexts := 1 << (hdr.Properties.LC + hdr.Properties.LP)
	d.literalProbs = make([]uint16, literalSize*literalContexts)

	for i := range d.isMatchProbs {
		d.isMatchProbs[i] = probInit
	}
	for i := range d.isRepProbs {
		d.isRepProbs[i] = probInit
		d.isRepG0Probs[i] = probInit
		d.isRepG1Probs[i] = probInit
		d.isRepG2Probs[i] = probInit
	}
	for i := range d.isRep0Long {
		d.isRep0Long[i] = probInit
	}
	for i := range d.literalProbs {
		d.literalProbs[i] = probInit
	}
	for i := range d.posSlotCoder {
		for j := range d.posSlotCoder[i] {
			d.posSlotCoder[i][j] = probInit
		}
	}
	for i := range d.posDecoders {
		d.posDecoders[i] = probInit
	}
	for i := range d.posAlign {
		d.posAlign[i] = probInit
	}

	d.lenDecoder.init()
	d.repLenDecoder.init()

	d.state = 0
	d.pos = 0
	d.prev = 0
	d.rep0 = 0
	d.rep1 = 0
	d.rep2 = 0
	d.rep3 = 0
	return nil
}

func (d *Decoder) currentPosState() int {
	mask := int64((1 << d.header.Properties.PB) - 1)
	return int(d.pos & mask)
}

func (d *Decoder) decodeLiteral() (byte, error) {
	hdr := d.header
	lpMask := int64((1 << hdr.Properties.LP) - 1)
	litCtx := int(((d.pos & lpMask) << hdr.Properties.LC) + int64(int(d.prev)>>uint(8-hdr.Properties.LC)))
	offset := litCtx * literalSize

	symbol := uint32(1)
	if d.state >= 7 {
		matchByte, err := d.out.getByte(d.rep0)
		if err != nil {
			return 0, err
		}
		for symbol < 0x100 {
			matchBit := (uint32(matchByte) >> 7) & 1
			matchByte <<= 1
			index := ((1 + matchBit) << 8) + symbol
			bit, err := d.rd.DecodeBit(&d.literalProbs[offset+int(index)])
			if err != nil {
				return 0, err
			}
			symbol = (symbol << 1) | bit
			if matchBit != bit {
				for symbol < 0x100 {
					bit, err := d.rd.DecodeBit(&d.literalProbs[offset+int(symbol)])
					if err != nil {
						return 0, err
					}
					symbol = (symbol << 1) | bit
				}
				break
			}
		}
	} else {
		for symbol < 0x100 {
			bit, err := d.rd.DecodeBit(&d.literalProbs[offset+int(symbol)])
			if err != nil {
				return 0, err
			}
			symbol = (symbol << 1) | bit
		}
	}

	return byte(symbol - 0x100), nil
}

func (d *Decoder) copyMatch(length uint32) error {
	for i := uint32(0); i < length; i++ {
		b, err := d.out.getByte(d.rep0)
		if err != nil {
			return err
		}
		if err := d.out.putByte(b); err != nil {
			return err
		}
		d.prev = b
		d.pos++
	}
	return nil
}

func decodeBitTree(rd *RangeDecoder, probs []uint16, numBits int) (uint32, error) {
	m := uint32(1)
	for i := 0; i < numBits; i++ {
		bit, err := rd.DecodeBit(&probs[m])
		if err != nil {
			return 0, err
		}
		m = (m << 1) | bit
	}
	return m - (1 << uint(numBits)), nil
}

func reverseDecodeBitTree(rd *RangeDecoder, probs []uint16, numBits int) (uint32, error) {
	var symbol uint32
	m := uint32(1)
	for i := 0; i < numBits; i++ {
		bit, err := rd.DecodeBit(&probs[m])
		if err != nil {
			return 0, err
		}
		m = (m << 1) | bit
		symbol |= bit << uint(i)
	}
	return symbol, nil
}

func reverseDecodeWithOffset(rd *RangeDecoder, probs []uint16, offset, numBits int) (uint32, error) {
	var symbol uint32
	m := uint32(1)
	for i := 0; i < numBits; i++ {
		bit, err := rd.DecodeBit(&probs[offset+int(m)])
		if err != nil {
			return 0, err
		}
		m = (m << 1) | bit
		symbol |= bit << uint(i)
	}
	return symbol, nil
}

func updateStateLiteral(state int) int {
	if state < 4 {
		return 0
	}
	if state < 10 {
		return state - 3
	}
	return state - 6
}

func updateStateMatch(state int) int {
	if state < 7 {
		return 7
	}
	return 10
}

func updateStateRep(state int) int {
	if state < 7 {
		return 8
	}
	return 11
}

func updateStateShortRep(state int) int {
	if state < 7 {
		return 9
	}
	return 11
}

func getLenToPosState(length uint32) int {
	length -= matchMinLen
	if length < numLenToPosStates {
		return int(length)
	}
	return numLenToPosStates - 1
}
