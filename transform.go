// Copyright (C) 2018 JT Olds
// See LICENSE for copying information.

package eestream

import (
	"bytes"
	"io"
	"io/ioutil"

	"github.com/jtolds/eestream/ranger"
)

// A Transformer is a data transformation that may change the size of the blocks
// of data it operates on in a deterministic fashion.
type Transformer interface {
	InBlockSize() int  // The block size prior to transformation
	OutBlockSize() int // The block size after transformation
	Transform(out, in []byte, blockNum int64) ([]byte, error)
}

type transformedReader struct {
	r        io.Reader
	t        Transformer
	blockNum int64
	inbuf    []byte
	outbuf   []byte
}

// TransformReader applies a Transformer to a Reader. startingBlockNum should
// probably be 0 unless you know you're already starting at a block offset.
func TransformReader(r io.Reader, t Transformer,
	startingBlockNum int64) io.Reader {
	return &transformedReader{
		r:        r,
		t:        t,
		blockNum: startingBlockNum,
		inbuf:    make([]byte, t.InBlockSize()),
		outbuf:   make([]byte, 0, t.OutBlockSize()),
	}
}

func (t *transformedReader) Read(p []byte) (n int, err error) {
	if len(t.outbuf) <= 0 {
		_, err = io.ReadFull(t.r, t.inbuf)
		if err != nil {
			return 0, err
		}
		t.outbuf, err = t.t.Transform(t.outbuf, t.inbuf, t.blockNum)
		if err != nil {
			return 0, Error.Wrap(err)
		}
		t.blockNum += 1
	}

	n = copy(p, t.outbuf)
	copy(t.outbuf, t.outbuf[n:])
	t.outbuf = t.outbuf[:len(t.outbuf)-n]
	return n, nil
}

type transformedRanger struct {
	rr ranger.Ranger
	t  Transformer
}

// Transform will apply a Transformer to a Ranger.
func Transform(rr ranger.Ranger, t Transformer) (ranger.Ranger, error) {
	if rr.Size()%int64(t.InBlockSize()) != 0 {
		return nil, Error.New("invalid transformer and range reader combination." +
			"the range reader size is not a multiple of the block size")
	}
	return &transformedRanger{rr: rr, t: t}, nil
}

func (t *transformedRanger) Size() int64 {
	blocks := t.rr.Size() / int64(t.t.InBlockSize())
	return blocks * int64(t.t.OutBlockSize())
}

func calcEncompassingBlocks(offset, length int64, blockSize int) (
	firstBlock, blockCount int64) {
	firstBlock = offset / int64(blockSize)
	if length <= 0 {
		return firstBlock, 0
	}
	lastBlock := (offset + length) / int64(blockSize)
	if (offset+length)%int64(blockSize) == 0 {
		return firstBlock, lastBlock - firstBlock
	}
	return firstBlock, 1 + lastBlock - firstBlock
}

func (t *transformedRanger) Range(offset, length int64) io.Reader {
	firstBlock, blockCount := calcEncompassingBlocks(
		offset, length, t.t.OutBlockSize())
	r := TransformReader(
		t.rr.Range(
			firstBlock*int64(t.t.InBlockSize()),
			blockCount*int64(t.t.InBlockSize())), t.t, firstBlock)
	_, err := io.CopyN(ioutil.Discard, r,
		offset-firstBlock*int64(t.t.OutBlockSize()))
	if err != nil {
		if err == io.EOF {
			return bytes.NewReader(nil)
		}
		return ranger.FatalReader(Error.Wrap(err))
	}
	return io.LimitReader(r, length)
}
