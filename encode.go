// Copyright (C) 2018 JT Olds
// See LICENSE for copying information.

package eestream

import (
	"io"
	"io/ioutil"
	"sync"

	"github.com/jtolds/eestream/ranger"
)

// ErasureScheme represents the general format of any erasure scheme algorithm.
// If this interface can be implemented, the rest of this library will work
// with it.
type ErasureScheme interface {
	// Encode will take 'in' and call 'out' with erasure coded pieces.
	Encode(in []byte, out func(num int, data []byte)) error

	// Decode will take a mapping of available erasure coded piece num -> data,
	// 'in', and append the combined data to 'out', returning it.
	Decode(out []byte, in map[int][]byte) ([]byte, error)

	// EncodedBlockSize is the size the erasure coded pieces should be that come
	// from Encode and are passed to Decode.
	EncodedBlockSize() int

	// DecodedBlockSize is the size the combined file blocks that should be
	// passed in to Encode and will come from Decode.
	DecodedBlockSize() int

	// Encode will generate this many pieces
	TotalCount() int

	// Decode requires at least this many pieces
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

// EncodeReader will take a Reader and an ErasureScheme and return a slice of
// Readers
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

// EncodedRanger will take an existing Ranger and provide a means to get
// multiple Ranged sub-Readers. EncodedRanger does not match the normal Ranger
// interface.
type EncodedRanger struct {
	es ErasureScheme
	rr ranger.Ranger
}

func NewEncodedRanger(rr ranger.Ranger, es ErasureScheme) (*EncodedRanger,
	error) {
	if rr.Size()%int64(es.DecodedBlockSize()) != 0 {
		return nil, Error.New("invalid erasure encoder and range reader combo. " +
			"range reader size must be a multiple of erasure encoder block size")
	}
	return &EncodedRanger{
		es: es,
		rr: rr,
	}, nil
}

// OutputSize is like Ranger.Size but returns the Size of the erasure encoded
// pieces that come out.
func (er *EncodedRanger) OutputSize() int64 {
	blocks := er.rr.Size() / int64(er.es.DecodedBlockSize())
	return blocks * int64(er.es.EncodedBlockSize())
}

// Range is like Ranger.Range, but returns a slice of Readers
func (er *EncodedRanger) Range(offset, length int64) ([]io.Reader, error) {
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
