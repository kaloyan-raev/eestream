package eestream

import (
	"io"
	"io/ioutil"
)

type decodedReader struct {
	rs     map[int]io.Reader
	es     ErasureScheme
	inbufs map[int][]byte
	outbuf []byte
	err    error
}

func DecodeReaders(rs map[int]io.Reader, es ErasureScheme) io.Reader {
	dr := &decodedReader{
		rs:     rs,
		es:     es,
		inbufs: make(map[int][]byte, len(rs)),
		outbuf: make([]byte, 0, es.DecodedBlockSize()),
	}
	for i := range rs {
		dr.inbufs[i] = make([]byte, es.EncodedBlockSize())
	}
	return dr
}

func (dr *decodedReader) Read(p []byte) (n int, err error) {
	if len(dr.outbuf) <= 0 {
		if dr.err != nil {
			return 0, err
		}
		errs := make(chan error, len(dr.rs))
		for i := range dr.rs {
			go func(i int) {
				_, err := io.ReadFull(dr.rs[i], dr.inbufs[i])
				errs <- err
			}(i)
		}
		for range dr.rs {
			err := <-errs
			if err != nil {
				dr.err = err
				return 0, err
			}
		}
		dr.outbuf, err = dr.es.Decode(dr.outbuf, dr.inbufs)
		if err != nil {
			return 0, err
		}
	}

	n = copy(p, dr.outbuf)
	copy(dr.outbuf, dr.outbuf[n:])
	dr.outbuf = dr.outbuf[:len(dr.outbuf)-n]
	return n, nil
}

type decodedRangeReader struct {
	es     ErasureScheme
	rrs    map[int]RangeReader
	inSize int64
}

func Decode(rrs map[int]RangeReader, es ErasureScheme) (RangeReader, error) {
	size := int64(-1)
	for _, rr := range rrs {
		if size == -1 {
			size = rr.Size()
		} else {
			if size != rr.Size() {
				return nil, Error.New("decode failure: range reader sizes don't " +
					"all match")
			}
		}
	}
	if size == -1 {
		return ByteRangeReader(nil), nil
	}
	if size%int64(es.EncodedBlockSize()) != 0 {
		return nil, Error.New("invalid erasure decoder and range reader combo. " +
			"range reader size must be a multiple of erasure encoder block size")
	}
	if len(rrs) < es.RequiredCount() {
		return nil, Error.New("not enough readers to reconstruct data!")
	}
	return &decodedRangeReader{
		es:     es,
		rrs:    rrs,
		inSize: size,
	}, nil
}

func (dr *decodedRangeReader) Size() int64 {
	blocks := dr.inSize / int64(dr.es.EncodedBlockSize())
	return blocks * int64(dr.es.DecodedBlockSize())
}

func (dr *decodedRangeReader) Range(offset, length int64) io.Reader {
	firstBlock, blockCount := calcEncompassingBlocks(
		offset, length, dr.es.DecodedBlockSize())

	readers := make(map[int]io.Reader, len(dr.rrs))
	for i, rr := range dr.rrs {
		readers[i] = rr.Range(
			firstBlock*int64(dr.es.EncodedBlockSize()),
			blockCount*int64(dr.es.EncodedBlockSize()))
	}
	r := DecodeReaders(readers, dr.es)
	_, err := io.CopyN(ioutil.Discard, r,
		offset-firstBlock*int64(dr.es.DecodedBlockSize()))
	if err != nil {
		return FatalReader(Error.Wrap(err))
	}
	return io.LimitReader(r, length)
}
