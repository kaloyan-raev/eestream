package eestream

import (
	"io"
	"io/ioutil"
	"sync"
)

type ErasureScheme interface {
	Encode(input []byte, output func(num int, data []byte)) error
	Decode(out []byte, in map[int][]byte) ([]byte, error)
	EncodedBlockSize() int
	DecodedBlockSize() int
	TotalCount() int
	RequiredCount() int
}

type encodedReader struct {
	r               io.Reader
	es              ErasureScheme
	cv              *sync.Cond
	inbuf           []byte
	outbufs         [][]byte
	piecesRemaining int
	err             error
}

func EncodeReader(r io.Reader, es ErasureScheme) []io.Reader {
	er := &encodedReader{
		r:       r,
		es:      es,
		cv:      sync.NewCond(&sync.Mutex{}),
		inbuf:   make([]byte, es.DecodedBlockSize()),
		outbufs: make([][]byte, es.TotalCount()),
	}
	readers := make([]io.Reader, 0, es.TotalCount())
	for i := 0; i < es.TotalCount(); i++ {
		er.outbufs[i] = make([]byte, 0, es.EncodedBlockSize())
		readers = append(readers, &encodedPiece{
			er: er,
			i:  i,
		})
	}
	return readers
}

func (er *encodedReader) wait() error {
	if er.err != nil {
		return er.err
	}
	if er.piecesRemaining > 0 {
		er.cv.Wait()
		return er.err
	}

	defer er.cv.Broadcast()
	_, err := io.ReadFull(er.r, er.inbuf)
	if err != nil {
		er.err = err
		return err
	}
	err = er.es.Encode(er.inbuf, func(num int, data []byte) {
		er.outbufs[num] = append(er.outbufs[num], data...)
	})
	if err != nil {
		er.err = err
		return err
	}
	er.piecesRemaining += er.es.TotalCount()
	return nil
}

type encodedPiece struct {
	er *encodedReader
	i  int
}

func (ep *encodedPiece) Read(p []byte) (n int, err error) {
	ep.er.cv.L.Lock()
	defer ep.er.cv.L.Unlock()

	outbufs, i := ep.er.outbufs, ep.i
	if len(outbufs[i]) <= 0 {
		err := ep.er.wait()
		if err != nil {
			return 0, err
		}
	}

	n = copy(p, outbufs[i])
	copy(outbufs[i], outbufs[i][n:])
	outbufs[i] = outbufs[i][:len(outbufs[i])-n]
	if len(outbufs[i]) <= 0 {
		ep.er.piecesRemaining -= 1
	}
	return n, nil
}

type EncodedRangeReader struct {
	es ErasureScheme
	rr RangeReader
}

func Encode(rr RangeReader, es ErasureScheme) (*EncodedRangeReader, error) {
	if rr.Size()%int64(es.DecodedBlockSize()) != 0 {
		return nil, Error.New("invalid erasure encoder and range reader combo. " +
			"range reader size must be a multiple of erasure encoder block size")
	}
	return &EncodedRangeReader{
		es: es,
		rr: rr,
	}, nil
}

func (er *EncodedRangeReader) OutputSize() int64 {
	blocks := er.rr.Size() / int64(er.es.DecodedBlockSize())
	return blocks * int64(er.es.EncodedBlockSize())
}

func (er *EncodedRangeReader) Range(offset, length int64) ([]io.Reader, error) {
	firstBlock, blockCount := calcEncompassingBlocks(
		offset, length, er.es.EncodedBlockSize())
	readers := EncodeReader(er.rr.Range(
		firstBlock*int64(er.es.DecodedBlockSize()),
		blockCount*int64(er.es.DecodedBlockSize())), er.es)

	for i, r := range readers {
		_, err := io.CopyN(ioutil.Discard, r,
			offset-firstBlock*int64(er.es.EncodedBlockSize()))
		if err != nil {
			return nil, Error.Wrap(err)
		}
		readers[i] = io.LimitReader(r, length)
	}
	return readers, nil
}
