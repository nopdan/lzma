package lzma

import "fmt"

// outWindow stores the sliding dictionary buffer used by LZMA match copying.
type outWindow struct {
	buf  []byte
	pos  int
	full int
	size int
}

// newOutWindow allocates a dictionary window with at least 1 byte.
func newOutWindow(size uint32) *outWindow {
	if size == 0 {
		size = 1
	}
	return &outWindow{
		buf:  make([]byte, size),
		size: int(size),
	}
}

// reset reinitializes dictionary position and resizes buffer if needed.
func (ow *outWindow) reset(size uint32) {
	if size == 0 {
		size = 1
	}
	required := int(size)
	if ow.buf == nil || cap(ow.buf) < required {
		ow.buf = make([]byte, size)
	}
	ow.size = required
	ow.pos = 0
	ow.full = 0
}

// putByte writes one byte into the circular dictionary.
func (ow *outWindow) putByte(b byte) {
	ow.buf[ow.pos] = b
	ow.pos++
	if ow.pos == ow.size {
		ow.pos = 0
	}
	if ow.full < ow.size {
		ow.full++
	}
}

// getByte reads a byte at the given match distance from current position.
func (ow *outWindow) getByte(distance uint32) (byte, error) {
	dist := int(distance)
	if dist >= ow.full {
		return 0, fmt.Errorf("invalid distance %d, only %d bytes available in window", distance, ow.full)
	}

	idx := ow.pos - dist - 1
	if idx < 0 {
		idx += ow.size
	}
	return ow.buf[idx], nil
}

// copyFrom copies match bytes into dictionary without exporting to caller buffer.
func (ow *outWindow) copyFrom(distance uint32, length uint32) error {
	_, err := ow.copyFromToDst(distance, length, nil)
	return err
}

// copyFromToDst copies match bytes into dictionary and mirrors them to dst.
func (ow *outWindow) copyFromToDst(distance uint32, length uint32, dst []byte) (uint32, error) {
	dist := int(distance)
	if dist >= ow.full {
		return 0, fmt.Errorf("invalid distance %d, only %d bytes available in window", distance, ow.full)
	}

	var outWritten int
	hasDst := dst != nil
	room := 0
	if hasDst {
		room = len(dst)
	}
	remaining := int(length)

	for remaining > 0 {
		if hasDst && room == 0 {
			break
		}

		src := ow.pos - dist - 1
		if src < 0 {
			src += ow.size
		}
		dstPos := ow.pos

		srcContig := ow.size - src
		dstContig := ow.size - dstPos

		chunk := min(min(remaining, srcContig), dstContig)
		// Overlap-safe copy: one step cannot exceed distance+1.
		maxSafe := dist + 1
		if chunk > maxSafe {
			chunk = maxSafe
		}
		if hasDst && chunk > room {
			chunk = room
		}
		if chunk <= 0 {
			break
		}

		copied := copy(ow.buf[dstPos:dstPos+chunk], ow.buf[src:src+chunk])
		if copied != chunk {
			chunk = copied
		}

		if hasDst {
			// Mirror produced bytes to caller while dictionary is updated.
			copy(dst[outWritten:outWritten+chunk], ow.buf[dstPos:dstPos+chunk])
			outWritten += chunk
			room -= chunk
		}

		ow.pos += chunk
		if ow.pos >= ow.size {
			ow.pos -= ow.size
		}
		if ow.full < ow.size {
			ow.full += chunk
			if ow.full > ow.size {
				ow.full = ow.size
			}
		}

		remaining -= chunk
	}

	return uint32(outWritten), nil
}
