package lzma

import (
	"errors"
	"fmt"
	"io"
)

var (
	// ErrNotImplemented is reserved for incomplete decode branches.
	ErrNotImplemented = errors.New("lzma decode path is not fully implemented yet")
	// ErrDataAfterEndMarker reports data that appears after an end marker in known-size mode.
	ErrDataAfterEndMarker = errors.New("lzma stream ended by end marker before expected size")
	// ErrOutputOverrun reports decoded output that exceeds the declared uncompressed size.
	ErrOutputOverrun = errors.New("lzma decoded output exceeds declared uncompressed size")
)

// Core LZMA decoder model constants.
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
)

// lenDecoder decodes match lengths from low/mid/high trees keyed by posState.
type lenDecoder struct {
	choice  uint16
	choice2 uint16
	low     [numPosStatesMax][1 << 3]uint16
	mid     [numPosStatesMax][1 << 3]uint16
	high    [1 << 8]uint16
}

// init resets all length decoder probabilities.
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

// decode decodes one length symbol for the given position state.
func (ld *lenDecoder) decode(rd *rangeDecoder, posState uint32) (uint32, error) {
	bit, err := rd.decodeBit(&ld.choice)
	if err != nil {
		return 0, err
	}
	if bit == 0 {
		return decodeBitTree(rd, ld.low[int(posState)][:], 3)
	}

	bit, err = rd.decodeBit(&ld.choice2)
	if err != nil {
		return 0, err
	}
	if bit == 0 {
		v, err := decodeBitTree(rd, ld.mid[int(posState)][:], 3)
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

// decoder holds all probability models and stream state for LZMA decoding.
type decoder struct {
	header header
	rd     *rangeDecoder
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

	state uint32
	pos   uint64
	prev  byte

	rep0 uint32
	rep1 uint32
	rep2 uint32
	rep3 uint32

	hasKnownSize bool
	knownSize    uint64

	posStateMask     uint32
	pb               uint32
	literalLPMask    uint32
	literalLC        uint32
	literalPrevShift uint32

	finished        bool
	pendingMatchLen uint32
}

// newDecoder allocates an empty decoder instance.
func newDecoder() *decoder {
	return &decoder{}
}

// resetForHeader resets per-stream state from parsed header metadata.
func (d *decoder) resetForHeader(hdr header) {
	d.header = hdr
	d.hasKnownSize = hdr.HasUncompressedSize
	if d.hasKnownSize {
		d.knownSize = hdr.UncompressedSize
	} else {
		d.knownSize = 0
	}
	d.finished = false
	d.pendingMatchLen = 0
	d.pos = 0
	d.prev = 0
	d.state = 0
	d.rep0 = 0
	d.rep1 = 0
	d.rep2 = 0
	d.rep3 = 0
}

// read decodes into p using pull mode and returns produced bytes.
func (d *decoder) read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}

	if d.finished {
		return 0, io.EOF
	}

	written := 0

	if d.pendingMatchLen > 0 {
		if err := d.emitPendingMatch(p, &written); err != nil {
			return written, err
		}
		if written >= len(p) {
			return written, nil
		}
	}

	for written < len(p) {
		if d.hasKnownSize && d.pos >= d.knownSize {
			d.finished = true
			if written == 0 {
				return 0, io.EOF
			}
			return written, nil
		}

		if err := d.decodeOne(p, &written); err != nil {
			if err == io.EOF {
				d.finished = true
				if written > 0 {
					return written, nil
				}
			}
			return written, err
		}

		if d.pendingMatchLen > 0 && written < len(p) {
			if err := d.emitPendingMatch(p, &written); err != nil {
				return written, err
			}
		}
	}

	if written == 0 && d.finished {
		return 0, io.EOF
	}

	return written, nil
}

// decodeOne decodes a single symbol (literal, match, or rep sequence).
func (d *decoder) decodeOne(dst []byte, written *int) error {
	if d.hasKnownSize && d.pos >= d.knownSize {
		d.finished = true
		return io.EOF
	}

	posState := d.currentPosState()
	state := d.state
	stateIdx := int(state)
	idx := (state << d.pb) + posState
	isMatch, err := d.rd.decodeBit(&d.isMatchProbs[int(idx)])
	if err != nil {
		return err
	}

	if isMatch == 0 {
		b, err := d.decodeLiteral()
		if err != nil {
			return err
		}
		d.out.putByte(b)
		dst[*written] = b
		*written = *written + 1
		d.state = updateStateLiteral(d.state)
		d.prev = b
		d.pos++
		return nil
	}

	isRep, err := d.rd.decodeBit(&d.isRepProbs[stateIdx])
	if err != nil {
		return err
	}

	var length uint32
	if isRep == 1 {
		isRepG0, err := d.rd.decodeBit(&d.isRepG0Probs[stateIdx])
		if err != nil {
			return err
		}
		if isRepG0 == 0 {
			isRep0LongIdx := (state << d.pb) + posState
			isRep0Long, err := d.rd.decodeBit(&d.isRep0Long[int(isRep0LongIdx)])
			if err != nil {
				return err
			}
			if isRep0Long == 0 {
				d.state = updateStateShortRep(d.state)
				b, err := d.out.getByte(d.rep0)
				if err != nil {
					return err
				}
				d.out.putByte(b)
				dst[*written] = b
				*written = *written + 1
				d.prev = b
				d.pos++
				return nil
			}
		} else {
			var dist uint32
			isRepG1, err := d.rd.decodeBit(&d.isRepG1Probs[stateIdx])
			if err != nil {
				return err
			}
			if isRepG1 == 0 {
				dist = d.rep1
			} else {
				isRepG2, err := d.rd.decodeBit(&d.isRepG2Probs[stateIdx])
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
		posSlot, err := decodeBitTree(d.rd, d.posSlotCoder[int(lenToPosState)][:], numPosSlotBits)
		if err != nil {
			return err
		}

		if posSlot < startPosModelIndex {
			d.rep0 = posSlot
		} else {
			numDirectBits := (posSlot >> 1) - 1
			d.rep0 = (2 | (posSlot & 1)) << numDirectBits

			if posSlot < endPosModelIndex {
				if d.rep0 < posSlot {
					return fmt.Errorf("invalid distance model offset: rep0=%d posSlot=%d", d.rep0, posSlot)
				}
				base := d.rep0 - posSlot
				extra, err := reverseDecodeWithOffset(d.rd, d.posDecoders, base, numDirectBits)
				if err != nil {
					return err
				}
				d.rep0 += extra
			} else {
				direct, err := d.rd.decodeDirectBits(numDirectBits - numAlignBits)
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

			// 0xFFFFFFFF is the end marker for unknown-size streams.
			if d.rep0 == ^uint32(0) {
				if !d.hasKnownSize {
					d.finished = true
					return io.EOF
				}
				return ErrDataAfterEndMarker
			}
		}
	}

	if d.hasKnownSize && d.pos+uint64(length) > d.knownSize {
		return ErrOutputOverrun
	}

	if err := d.copyMatchTo(length, dst, written); err != nil {
		return err
	}

	return nil
}

// initModels allocates and initializes all probability tables for current properties.
func (d *decoder) initModels() error {
	hdr := d.header
	if hdr.Properties.LC > 8 || hdr.Properties.LP > 4 || hdr.Properties.PB > 4 {
		return ErrInvalidProperties
	}

	propLC := uint32(hdr.Properties.LC)
	propLP := uint32(hdr.Properties.LP)
	propPB := uint32(hdr.Properties.PB)
	posStates := 1 << hdr.Properties.PB
	literalContexts := 1 << (hdr.Properties.LC + hdr.Properties.LP)
	requiredMatchLen := numStates * posStates
	requiredLiteralLen := literalSize * literalContexts
	requiredPosStatesLen := numStates * posStates
	requiredPosDecodersLen := numFullDistances - startPosModelIndex

	if cap(d.isMatchProbs) < requiredMatchLen {
		d.isMatchProbs = make([]uint16, requiredMatchLen)
	} else {
		d.isMatchProbs = d.isMatchProbs[:requiredMatchLen]
	}

	if len(d.isRepProbs) != numStates {
		d.isRepProbs = make([]uint16, numStates)
	}
	if len(d.isRepG0Probs) != numStates {
		d.isRepG0Probs = make([]uint16, numStates)
	}
	if len(d.isRepG1Probs) != numStates {
		d.isRepG1Probs = make([]uint16, numStates)
	}
	if len(d.isRepG2Probs) != numStates {
		d.isRepG2Probs = make([]uint16, numStates)
	}

	if cap(d.isRep0Long) < requiredPosStatesLen {
		d.isRep0Long = make([]uint16, requiredPosStatesLen)
	} else {
		d.isRep0Long = d.isRep0Long[:requiredPosStatesLen]
	}

	if cap(d.literalProbs) < requiredLiteralLen {
		d.literalProbs = make([]uint16, requiredLiteralLen)
	} else {
		d.literalProbs = d.literalProbs[:requiredLiteralLen]
	}

	if len(d.posDecoders) != requiredPosDecodersLen {
		d.posDecoders = make([]uint16, requiredPosDecodersLen)
	}

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

	d.posStateMask = (uint32(1) << propPB) - 1
	d.pb = propPB
	d.literalLPMask = (uint32(1) << propLP) - 1
	d.literalLC = propLC
	d.literalPrevShift = 8 - propLC

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

// currentPosState returns the masked position state derived from output position.
func (d *decoder) currentPosState() uint32 {
	return uint32(d.pos) & d.posStateMask
}

// decodeLiteral decodes a literal byte using plain or matched literal coding.
func (d *decoder) decodeLiteral() (byte, error) {
	pos := uint32(d.pos)
	prev := uint32(d.prev)
	litCtx := ((pos & d.literalLPMask) << d.literalLC) + (prev >> d.literalPrevShift)
	probs := d.literalProbs[int(litCtx)*literalSize:]

	if d.state >= 7 {
		matchByte, err := d.out.getByte(d.rep0)
		if err != nil {
			return 0, err
		}
		return d.decodeLiteralMatched(probs, matchByte)
	}

	return d.decodeLiteralPlain(probs)
}

// decodeLiteralPlain decodes a literal without match-byte context.
func (d *decoder) decodeLiteralPlain(probs []uint16) (byte, error) {
	symbol := uint32(1)
	for symbol < 0x100 {
		bit, err := d.rd.decodeBit(&probs[int(symbol)])
		if err != nil {
			return 0, err
		}
		symbol = (symbol << 1) | bit
	}
	return byte(symbol - 0x100), nil
}

// decodeLiteralMatched decodes a literal with match-byte guided probability selection.
func (d *decoder) decodeLiteralMatched(probs []uint16, matchByte byte) (byte, error) {
	symbol := uint32(1)
	mb := uint32(matchByte)
	for symbol < 0x100 {
		matchBit := (mb >> 7) & 1
		mb <<= 1
		idx := ((1 + matchBit) << 8) | symbol
		bit, err := d.rd.decodeBit(&probs[int(idx)])
		if err != nil {
			return 0, err
		}
		symbol = (symbol << 1) | bit
		if matchBit != bit {
			break
		}
	}

	for symbol < 0x100 {
		bit, err := d.rd.decodeBit(&probs[int(symbol)])
		if err != nil {
			return 0, err
		}
		symbol = (symbol << 1) | bit
	}

	return byte(symbol - 0x100), nil
}

// copyMatchTo copies a decoded match into dictionary and optional output buffer.
func (d *decoder) copyMatchTo(length uint32, dst []byte, written *int) error {
	if written == nil {
		if err := d.out.copyFrom(d.rep0, length); err != nil {
			return err
		}
		d.pos += uint64(length)
		if b, err := d.out.getByte(0); err == nil {
			d.prev = b
		}
		d.pendingMatchLen = 0
		return nil
	}

	// If dst is full, remaining bytes are emitted by emitPendingMatch on next Read call.
	produced, err := d.out.copyFromToDst(d.rep0, length, dst[*written:])
	if err != nil {
		return err
	}
	*written += int(produced)
	d.pos += uint64(produced)
	if produced > 0 {
		if b, err := d.out.getByte(0); err == nil {
			d.prev = b
		}
	}
	d.pendingMatchLen = length - produced
	return nil
}

// emitPendingMatch flushes remaining match bytes from previous partial copy.
func (d *decoder) emitPendingMatch(dst []byte, written *int) error {
	if d.pendingMatchLen == 0 {
		return nil
	}

	produced, err := d.out.copyFromToDst(d.rep0, d.pendingMatchLen, dst[*written:])
	if err != nil {
		return err
	}
	*written += int(produced)
	d.pos += uint64(produced)
	if produced > 0 {
		if b, err := d.out.getByte(0); err == nil {
			d.prev = b
		}
	}
	d.pendingMatchLen -= produced
	return nil
}

// decodeBitTree decodes a forward bit-tree symbol.
func decodeBitTree(rd *rangeDecoder, probs []uint16, numBits uint32) (uint32, error) {
	m := uint32(1)
	for range numBits {
		bit, err := rd.decodeBit(&probs[int(m)])
		if err != nil {
			return 0, err
		}
		m = (m << 1) | bit
	}
	return m - (uint32(1) << numBits), nil
}

// reverseDecodeBitTree decodes a reverse bit-tree symbol.
func reverseDecodeBitTree(rd *rangeDecoder, probs []uint16, numBits uint32) (uint32, error) {
	var symbol uint32
	m := uint32(1)
	for i := range numBits {
		bit, err := rd.decodeBit(&probs[int(m)])
		if err != nil {
			return 0, err
		}
		m = (m << 1) | bit
		symbol |= bit << i
	}
	return symbol, nil
}

// reverseDecodeWithOffset decodes a reverse bit-tree symbol from probs[offset:].
func reverseDecodeWithOffset(rd *rangeDecoder, probs []uint16, offset, numBits uint32) (uint32, error) {
	var symbol uint32
	m := uint32(1)
	for i := range numBits {
		bit, err := rd.decodeBit(&probs[int(offset+m)])
		if err != nil {
			return 0, err
		}
		m = (m << 1) | bit
		symbol |= bit << i
	}
	return symbol, nil
}

// updateStateLiteral applies the LZMA state transition after a literal.
func updateStateLiteral(state uint32) uint32 {
	if state < 4 {
		return 0
	}
	if state < 10 {
		return state - 3
	}
	return state - 6
}

// updateStateMatch applies the LZMA state transition after a new match.
func updateStateMatch(state uint32) uint32 {
	if state < 7 {
		return 7
	}
	return 10
}

// updateStateRep applies the LZMA state transition after a repeated match.
func updateStateRep(state uint32) uint32 {
	if state < 7 {
		return 8
	}
	return 11
}

// updateStateShortRep applies the LZMA state transition after a short rep.
func updateStateShortRep(state uint32) uint32 {
	if state < 7 {
		return 9
	}
	return 11
}

// getLenToPosState maps match length to the distance-slot state.
func getLenToPosState(length uint32) uint32 {
	length -= matchMinLen
	if length < uint32(numLenToPosStates) {
		return length
	}
	return uint32(numLenToPosStates - 1)
}
