package eestream

import (
	"bytes"
	"io"
)

type RangeReader interface {
	Size() int64
	Range(offset, length int64) io.Reader
}

func FatalReader(err error) io.Reader {
	return &fatalReader{Err: err}
}

type fatalReader struct {
	Err error
}

func (f *fatalReader) Read(p []byte) (n int, err error) {
	return 0, f.Err
}

type ByteRangeReader []byte

func (b ByteRangeReader) Size() int64 { return int64(len(b)) }

func (b ByteRangeReader) Range(offset, length int64) io.Reader {
	if offset < 0 {
		return FatalReader(Error.New("negative offset"))
	}
	if offset+length > int64(len(b)) {
		return FatalReader(Error.New("buffer runoff"))
	}

	return bytes.NewReader(b[offset : offset+length])
}

type concatReader struct {
	r1 RangeReader
	r2 RangeReader
}

func (c *concatReader) Size() int64 {
	return c.r1.Size() + c.r2.Size()
}

func (c *concatReader) Range(offset, length int64) io.Reader {
	r1Size := c.r1.Size()
	if offset+length <= r1Size {
		return c.r1.Range(offset, length)
	}
	if offset >= r1Size {
		return c.r2.Range(offset-r1Size, length)
	}
	return io.MultiReader(
		c.r1.Range(offset, r1Size-offset),
		LazyReader(func() io.Reader {
			return c.r2.Range(0, length-(r1Size-offset))
		}))
}

func concat2(r1, r2 RangeReader) RangeReader {
	return &concatReader{r1: r1, r2: r2}
}

func Concat(r ...RangeReader) RangeReader {
	switch len(r) {
	case 0:
		return ByteRangeReader(nil)
	case 1:
		return r[0]
	case 2:
		return concat2(r[0], r[1])
	default:
		mid := len(r) / 2
		return concat2(Concat(r[:mid]...), Concat(r[mid:]...))
	}
}

type lazyReader struct {
	fn func() io.Reader
	r  io.Reader
}

func LazyReader(reader func() io.Reader) io.Reader {
	return &lazyReader{fn: reader}
}

func (l *lazyReader) Read(p []byte) (n int, err error) {
	if l.r == nil {
		l.r = l.fn()
		l.fn = nil
	}
	return l.r.Read(p)
}

type subrange struct {
	r              RangeReader
	offset, length int64
}

func Subrange(data RangeReader, offset, length int64) (RangeReader, error) {
	dSize := data.Size()
	if offset < 0 || offset > dSize {
		return nil, Error.New("invalid offset")
	}
	if length+offset > dSize {
		return nil, Error.New("invalid length")
	}
	return &subrange{r: data, offset: offset, length: length}, nil
}

func (s *subrange) Size() int64 {
	return s.length
}

func (s *subrange) Range(offset, length int64) io.Reader {
	return s.r.Range(offset+s.offset, length)
}
