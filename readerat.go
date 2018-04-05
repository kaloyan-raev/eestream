package eestream

import (
	"fmt"
	"io"
)

type readerAtRangeReader struct {
	r    io.ReaderAt
	size int64
}

func ReaderAtRangeReader(r io.ReaderAt, size int64) RangeReader {
	return &readerAtRangeReader{
		r:    r,
		size: size,
	}
}

func (r *readerAtRangeReader) Size() int64 {
	return r.size
}

type readerAtReader struct {
	r              io.ReaderAt
	offset, length int64
}

func (r *readerAtRangeReader) Range(offset, length int64) io.Reader {
	if offset < 0 {
		return FatalReader(fmt.Errorf("negative offset"))
	}
	if offset+length > r.size {
		return FatalReader(fmt.Errorf("buffer runoff"))
	}
	return &readerAtReader{r: r.r, offset: offset, length: length}
}

func (r *readerAtReader) Read(p []byte) (n int, err error) {
	if r.length == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.length {
		p = p[:r.length]
	}
	n, err = r.r.ReadAt(p, r.offset)
	r.offset += int64(n)
	r.length -= int64(n)
	return n, err
}
