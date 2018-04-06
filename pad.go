package eestream

import (
	"bytes"
	"fmt"
	"io"
)

func Pad(data RangeReader, blockSize int) (
	rr RangeReader, padding int, err error) {
	if blockSize >= 256 {
		return nil, 0, fmt.Errorf("blockSize too large")
	}
	r := data.Size() % int64(blockSize)
	padding = blockSize - int(r)
	paddingBytes := bytes.Repeat([]byte{byte(padding)}, padding)
	return Concat(data, ByteRangeReader(paddingBytes)), padding, nil
}

func Unpad(data RangeReader, padding int) (RangeReader, error) {
	return Subrange(data, 0, data.Size()-int64(padding))
}

func UnpadSlow(data RangeReader) (RangeReader, error) {
	var p [1]byte
	_, err := io.ReadFull(data.Range(data.Size()-1, 1), p[:])
	if err != nil {
		return nil, err
	}
	return Unpad(data, int(p[0]))
}
