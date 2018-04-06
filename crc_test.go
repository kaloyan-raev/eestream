// Copyright (C) 2018 JT Olds
// See LICENSE for copying information.

package eestream

import (
	"encoding/binary"
	"hash/crc32"

	"github.com/jtolds/eestream/ranger"
)

type crcAdder struct {
	Table *crc32.Table
}

func newCRCAdder(t *crc32.Table) *crcAdder {
	return &crcAdder{Table: t}
}

func (c *crcAdder) InBlockSize() int  { return 64 }
func (c *crcAdder) OutBlockSize() int { return 64 + 4 + 8 }

func (c *crcAdder) Transform(out, in []byte, blockOffset int64) (
	[]byte, error) {
	out = append(out, in...)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(blockOffset))
	out = append(out, buf[:]...)
	binary.BigEndian.PutUint32(buf[:4], crc32.Checksum(out, c.Table))
	out = append(out, buf[:4]...)
	return out, nil
}

type crcChecker struct {
	Table *crc32.Table
}

func newCRCChecker(t *crc32.Table) *crcChecker {
	return &crcChecker{Table: t}
}

func (c *crcChecker) InBlockSize() int  { return 64 + 4 + 8 }
func (c *crcChecker) OutBlockSize() int { return 64 }

func (c *crcChecker) Transform(out, in []byte, blockOffset int64) (
	[]byte, error) {
	bs := c.OutBlockSize()
	if binary.BigEndian.Uint32(in[bs+8:bs+8+4]) !=
		crc32.Checksum(in[:bs+8], c.Table) {
		return nil, Error.New("crc check mismatch")
	}
	if binary.BigEndian.Uint64(in[bs:bs+8]) != uint64(blockOffset) {
		return nil, Error.New("block offset mismatch")
	}
	return append(out, in[:bs]...), nil
}

func addCRC(data ranger.Ranger, tab *crc32.Table) (ranger.Ranger, error) {
	return Transform(data, newCRCAdder(tab))
}

func checkCRC(data ranger.Ranger, tab *crc32.Table) (ranger.Ranger, error) {
	return Transform(data, newCRCChecker(tab))
}
