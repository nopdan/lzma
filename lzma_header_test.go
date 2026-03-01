package lzma

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"testing"
)

func TestParseProperties(t *testing.T) {
	tests := []struct {
		name    string
		in      byte
		want    properties
		wantErr error
	}{
		{
			name: "zero",
			in:   0,
			want: properties{LC: 0, LP: 0, PB: 0},
		},
		{
			name: "common-5d",
			in:   0x5d,
			want: properties{LC: 3, LP: 0, PB: 2},
		},
		{
			name:    "invalid",
			in:      225,
			wantErr: ErrInvalidProperties,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseProperties(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ParseProperties(%d) error = %v, want %v", tt.in, err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseProperties(%d) returned error: %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("ParseProperties(%d) = %+v, want %+v", tt.in, got, tt.want)
			}
		})
	}
}

func TestParseLZMAHeader(t *testing.T) {
	var raw [lzmaHeaderSize]byte
	raw[0] = 0x5d
	binary.LittleEndian.PutUint32(raw[1:5], 0)
	binary.LittleEndian.PutUint64(raw[5:13], 12345)

	hdr, err := parseLZMAHeader(raw)
	if err != nil {
		t.Fatalf("ParseLZMAHeader returned error: %v", err)
	}
	if hdr.Properties.LC != 3 || hdr.Properties.LP != 0 || hdr.Properties.PB != 2 {
		t.Fatalf("unexpected properties: %+v", hdr.Properties)
	}
	if hdr.DictionarySize != 1 {
		t.Fatalf("unexpected dictionary size normalization: %d", hdr.DictionarySize)
	}
	if hdr.UncompressedSize != 12345 {
		t.Fatalf("unexpected uncompressed size: %d", hdr.UncompressedSize)
	}
	if !hdr.HasUncompressedSize {
		t.Fatalf("expected known uncompressed size")
	}
}

func TestReadLZMAHeaderUnknownSize(t *testing.T) {
	var raw [lzmaHeaderSize]byte
	raw[0] = 0x5d
	binary.LittleEndian.PutUint32(raw[1:5], 4096)
	binary.LittleEndian.PutUint64(raw[5:13], ^uint64(0))

	hdr, err := readLZMAHeader(bytes.NewReader(raw[:]))
	if err != nil {
		t.Fatalf("ReadLZMAHeader returned error: %v", err)
	}
	if hdr.HasUncompressedSize {
		t.Fatalf("expected unknown size flag")
	}
	if hdr.UncompressedSize != 0 {
		t.Fatalf("expected zero value for unknown size, got: %d", hdr.UncompressedSize)
	}
}

func TestReadLZMAHeaderShortInput(t *testing.T) {
	_, err := readLZMAHeader(bytes.NewReader([]byte{0x5d, 0x00}))
	if !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("readLZMAHeader short input error = %v, want %v", err, ErrInvalidHeader)
	}
}

func TestReadLZMAHeaderReadError(t *testing.T) {
	reader := io.LimitReader(bytes.NewReader(make([]byte, 10)), 10)
	_, err := readLZMAHeader(reader)
	if !errors.Is(err, ErrInvalidHeader) {
		t.Fatalf("readLZMAHeader limited reader error = %v, want %v", err, ErrInvalidHeader)
	}
}
