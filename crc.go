package eestream

import (
	"encoding/binary"
	"fmt"
	"hash/crc32"
)

type CRCAdder struct {
	Table *crc32.Table
}

func (c *CRCAdder) BlockSize() int     { return 64 }
func (c *CRCAdder) BlockOverhead() int { return 4 + 8 }

func (c *CRCAdder) Transform(out, in []byte, blockOffset int64) (
	[]byte, error) {
	out = append(out, in...)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(blockOffset))
	out = append(out, buf[:]...)
	binary.BigEndian.PutUint32(buf[:4], crc32.Checksum(out, c.Table))
	out = append(out, buf[:4]...)
	return out, nil
}

type CRCChecker struct {
	Table *crc32.Table
}

func (c *CRCChecker) BlockSize() int     { return 64 + 4 + 8 }
func (c *CRCChecker) BlockOverhead() int { return -(4 + 8) }

func (c *CRCChecker) Transform(out, in []byte, blockOffset int64) (
	[]byte, error) {
	bs := c.BlockSize() + c.BlockOverhead()
	if binary.BigEndian.Uint32(in[bs+8:bs+8+4]) !=
		crc32.Checksum(in[:bs+8], c.Table) {
		return nil, fmt.Errorf("crc check mismatch")
	}
	if binary.BigEndian.Uint64(in[bs:bs+8]) != uint64(blockOffset) {
		return nil, fmt.Errorf("block offset mismatch")
	}
	return append(out, in[:bs]...), nil
}

func AddCRC(data RangeReader, tab *crc32.Table) (RangeReader, error) {
	return Transform(&CRCAdder{Table: tab}, data)
}

func CheckCRC(data RangeReader, tab *crc32.Table) (RangeReader, error) {
	return Transform(&CRCChecker{Table: tab}, data)
}
