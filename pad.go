package eestream

import (
	"bytes"
	"encoding/binary"
	"io"
)

const (
	uint32Size = 4
)

func Padding(dataLen int64, blockSize int) []byte {
	amount := dataLen + uint32Size
	r := amount % int64(blockSize)
	padding := uint32Size
	if r > 0 {
		padding += blockSize - int(r)
	}
	paddingBytes := bytes.Repeat([]byte{0}, padding)
	binary.BigEndian.PutUint32(paddingBytes[padding-uint32Size:], uint32(padding))
	return paddingBytes
}

func Pad(data RangeReader, blockSize int) (
	rr RangeReader, padding int) {
	paddingBytes := Padding(data.Size(), blockSize)
	return Concat(data, ByteRangeReader(paddingBytes)), len(paddingBytes)
}

func Unpad(data RangeReader, padding int) (RangeReader, error) {
	return Subrange(data, 0, data.Size()-int64(padding))
}

func UnpadSlow(data RangeReader) (RangeReader, error) {
	var p [uint32Size]byte
	_, err := io.ReadFull(data.Range(data.Size()-uint32Size, uint32Size), p[:])
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return Unpad(data, int(binary.BigEndian.Uint32(p[:])))
}

func PadReader(data io.Reader, blockSize int) io.Reader {
	cr := NewCountingReader(data)
	return io.MultiReader(cr, LazyReader(func() io.Reader {
		return bytes.NewReader(Padding(cr.N, blockSize))
	}))
}

type CountingReader struct {
	R io.Reader
	N int64
}

func NewCountingReader(r io.Reader) *CountingReader {
	return &CountingReader{R: r}
}

func (cr *CountingReader) Read(p []byte) (n int, err error) {
	n, err = cr.R.Read(p)
	cr.N += int64(n)
	return n, err
}
