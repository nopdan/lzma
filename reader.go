package lzma

import (
	"io"
	"sync"
)

// decoderPool reuses Decoder instances to reduce allocations across readers.
var decoderPool = sync.Pool{
	New: func() any {
		return newDecoder()
	},
}

// Reader provides io.Reader-compatible LZMA decompression.
type Reader struct {
	r        io.Reader
	dec      *decoder
	err      error
	closed   bool
	inited   bool
	released bool
}

// NewReader parses LZMA header and creates a stream decoder.
func NewReader(r io.Reader) (*Reader, error) {
	hdr, err := readLZMAHeader(r)
	if err != nil {
		return nil, err
	}

	dec := decoderPool.Get().(*decoder)
	dec.resetForHeader(hdr)

	// Known-size stream with size 0 must immediately return EOF.
	if hdr.HasUncompressedSize && hdr.UncompressedSize == 0 {
		dec.finished = true
		zr := &Reader{
			dec:    dec,
			closed: true,
			inited: true,
		}
		zr.release()
		return zr, nil
	}

	return &Reader{
		r:   r,
		dec: dec,
	}, nil
}

// init lazily initializes range decoder, probability models, and output window.
func (r *Reader) init() error {
	if r.inited {
		return nil
	}
	r.inited = true

	if r.dec.rd == nil {
		rd, err := newRangeDecoder(r.r)
		if err != nil {
			return err
		}
		r.dec.rd = rd
	} else {
		if err := r.dec.rd.reset(r.r); err != nil {
			return err
		}
	}

	if err := r.dec.initModels(); err != nil {
		return err
	}

	if r.dec.out == nil {
		r.dec.out = newOutWindow(r.dec.header.DictionarySize)
	} else {
		r.dec.out.reset(r.dec.header.DictionarySize)
	}
	return nil
}

// release returns decoder resources back to the pool once.
func (r *Reader) release() {
	if r.released || r.dec == nil {
		return
	}
	if r.dec.rd != nil {
		r.dec.rd.release()
	}
	r.dec.resetForHeader(header{})
	decoderPool.Put(r.dec)
	r.released = true
}

// Read decodes bytes into p and follows io.Reader EOF/error semantics.
func (r *Reader) Read(p []byte) (int, error) {
	if r.closed {
		return 0, io.EOF
	}
	if r.err != nil {
		return 0, r.err
	}

	if err := r.init(); err != nil {
		r.err = err
		r.release()
		return 0, err
	}

	n, err := r.dec.read(p)
	if err != nil {
		r.err = err
		r.closed = true
		r.release()
	}
	return n, err
}

// Close stops reading and releases pooled decoder resources.
func (r *Reader) Close() error {
	r.closed = true
	r.release()
	return nil
}
