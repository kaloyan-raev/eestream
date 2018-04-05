package eestream

import (
	"fmt"
	"io"
	"io/ioutil"
)

type Transformer interface {
	BlockSize() int
	BlockOverhead() int
	Transform(out, in []byte, blockOffset int64) ([]byte, error)
}

type transformer struct {
	t Transformer
	r RangeReader
}

func Transform(t Transformer, r RangeReader) (RangeReader, error) {
	if r.Size()%int64(t.BlockSize()) != 0 {
		return nil, fmt.Errorf("invalid transformer and range reader combination." +
			"the range reader size is not a multiple of the block size")
	}
	return &transformer{t: t, r: r}, nil
}

func (t *transformer) Size() int64 {
	blocks := t.r.Size() / int64(t.t.BlockSize())
	return int64(blocks) * (int64(t.t.BlockSize()) + int64(t.t.BlockOverhead()))
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

func (t *transformer) Range(offset, length int64) io.Reader {
	preBlockSize := t.t.BlockSize()
	postBlockSize := preBlockSize + t.t.BlockOverhead()
	firstBlock, blockCount := calcEncompassingBlocks(
		offset, length, postBlockSize)
	r := &transformReader{
		firstBlock: firstBlock,
		blockCount: blockCount,
		pre:        make([]byte, preBlockSize),
		post:       make([]byte, 0, postBlockSize),
		t:          t.t,
		r: t.r.Range(
			firstBlock*int64(preBlockSize),
			blockCount*int64(preBlockSize)),
	}
	_, err := io.CopyN(ioutil.Discard, r, offset-firstBlock*int64(postBlockSize))
	if err != nil {
		return FatalReader(err)
	}
	return io.LimitReader(r, length)
}

type transformReader struct {
	firstBlock int64
	blockCount int64
	pre, post  []byte
	t          Transformer
	r          io.Reader
}

func (t *transformReader) Read(p []byte) (n int, err error) {
	if len(t.post) <= 0 {
		if t.blockCount <= 0 {
			return 0, io.EOF
		}
		_, err = io.ReadFull(t.r, t.pre)
		if err != nil {
			return 0, err
		}
		t.post, err = t.t.Transform(t.post, t.pre, t.firstBlock)
		if err != nil {
			return 0, err
		}
		t.firstBlock += 1
		t.blockCount -= 1
	}

	n = copy(p, t.post)
	copy(t.post, t.post[n:])
	t.post = t.post[:len(t.post)-n]
	return n, nil
}
