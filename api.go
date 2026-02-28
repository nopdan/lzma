package lzma

import (
	"bytes"
)

// Decompress decompresses a complete LZMA (.lzma) byte stream into memory.
func Decompress(src []byte) ([]byte, error) {
	dec := NewDecoder()
	var out bytes.Buffer

	if len(src) >= LZMAHeaderSize {
		var raw [LZMAHeaderSize]byte
		copy(raw[:], src[:LZMAHeaderSize])
		if hdr, err := ParseLZMAHeader(raw); err == nil && hdr.UncompressedSize > 0 {
			maxInt := int64(^uint(0) >> 1)
			if hdr.UncompressedSize <= maxInt {
				out.Grow(int(hdr.UncompressedSize))
			}
		}
	}

	if err := dec.DecodeLZMA(bytes.NewReader(src), &out); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
