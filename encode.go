package eestream

import (
	"fmt"
	"io"
	"io/ioutil"
	"sync"
)

type ErasureEncoder interface {
	Encode(input []byte, output func(num int, data []byte)) error
	BlockSize() int
	OutputSize() int
	OutputCount() int
}

func NewErasureEncode(ee ErasureEncoder, rr RangeReader) (
	*ErasureReader, error) {
	if rr.Size()%int64(ee.BlockSize()) != 0 {
		return nil, fmt.Errorf("invalid erasure encoder and range reader combo. " +
			"range reader size must be a multiple of erasure encoder block size")
	}
	return &ErasureReader{
		rr: rr,
		ee: ee,
	}, nil
}

type ErasureReader struct {
	rr RangeReader
	ee ErasureEncoder
}

func (er *ErasureReader) OutputSize() int64 {
	blocks := er.rr.Size() / int64(er.ee.BlockSize())
	return blocks * int64(er.ee.OutputSize())
}

func (er *ErasureReader) Range(offset, length int64) ([]io.Reader, error) {
	preBlockSize := er.ee.BlockSize()
	postBlockSize := er.ee.OutputSize()
	firstBlock, blockCount := calcEncompassingBlocks(offset, length, postBlockSize)
	erange := &erasureRange{
		cv:         sync.NewCond(&sync.Mutex{}),
		firstBlock: firstBlock,
		blockCount: blockCount,
		pre:        make([]byte, preBlockSize),
		post:       make([][]byte, 0, er.ee.OutputCount()),
		ee:         er.ee,
		r: er.rr.Range(
			firstBlock*int64(preBlockSize),
			blockCount*int64(preBlockSize)),
	}
	readers := make([]io.Reader, 0, er.ee.OutputCount())
	for i := 0; i < er.ee.OutputCount(); i++ {
		erange.post[i] = make([]byte, 0, postBlockSize)
		r := &erasurePiece{
			erange: erange,
			piece:  i,
		}
		_, err := io.CopyN(ioutil.Discard, r, offset-firstBlock*int64(postBlockSize))
		if err != nil {
			return nil, err
		}
		readers = append(readers, io.LimitReader(r, length))
	}
	return readers, nil
}

type erasureRange struct {
	cv            *sync.Cond
	firstBlock    int64
	blockCount    int64
	pre           []byte
	post          [][]byte
	ee            ErasureEncoder
	r             io.Reader
	err           error
	dataRemaining int
}

type erasurePiece struct {
	erange *erasureRange
	piece  int
}

func (ep *erasurePiece) Read(p []byte) (n int, err error) {
	ep.erange.cv.L.Lock()
	defer ep.erange.cv.L.Unlock()

	post, i := ep.erange.post, ep.piece
	if len(post[i]) <= 0 {
		if ep.erange.err != nil {
			return 0, ep.erange.err
		}
		if ep.erange.blockCount <= 0 {
			return 0, io.EOF
		}
		if ep.erange.dataRemaining > 0 {
			ep.erange.cv.Wait()
		} else {
			_, err = io.ReadFull(ep.erange.r, ep.erange.pre)
			if err != nil {
				ep.erange.err = err
				return 0, err
			}
			err = ep.erange.ee.Encode(ep.erange.pre, func(piece int, data []byte) {
				post[piece] = append(post[piece], data...)
			})
			if err != nil {
				ep.erange.err = err
				return 0, err
			}
			ep.erange.firstBlock += 1
			ep.erange.blockCount -= 1
			ep.erange.dataRemaining += ep.erange.ee.OutputCount()
			ep.erange.cv.Broadcast()
		}
	}

	n = copy(p, post[i])
	copy(post[i], post[i][n:])
	post[i] = post[i][:len(post[i])-n]
	if len(post[i]) <= 0 {
		ep.erange.dataRemaining -= 1
	}
	return n, nil
}
